package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/sambady/heretous/internal/config"
)

func TestHTTPServerLifecycle(t *testing.T) {
	a := newTestApp(time.Second)
	url, cancel, done := startTestApp(t, a)
	client := newTestClient(t)

	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed},
	} {
		req, err := http.NewRequest(tc.method, url+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.status {
			t.Errorf("%s %s: status = %d, want %d", tc.method, tc.path, resp.StatusCode, tc.status)
		}
		if tc.path == "/healthz" && tc.method == http.MethodGet {
			if string(body) != "{\"status\":\"ok\"}\n" || resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("unexpected health response: headers=%v body=%q", resp.Header, body)
			}
		}
	}

	cancel()
	if err := awaitResult(t, done); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	resp, err := client.Get(url + "/healthz")
	if err == nil {
		resp.Body.Close()
		t.Fatal("server accepted a request after shutdown")
	}
}

func TestShutdownWaitsForActiveRequest(t *testing.T) {
	a := newTestApp(time.Second)
	started := make(chan struct{})
	release := make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	t.Cleanup(finish)
	a.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte("finished"))
	})
	shutdownStarted := make(chan struct{})
	a.server.RegisterOnShutdown(func() { close(shutdownStarted) })
	url, cancel, done := startTestApp(t, a)
	client := newTestClient(t)
	requestDone := make(chan error, 1)
	go func() {
		resp, err := client.Get(url)
		if err == nil {
			var body []byte
			body, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil && string(body) != "finished" {
				err = errors.New("active response was not completed")
			}
		}
		requestDone <- err
	}()

	awaitSignal(t, started)
	cancel()
	awaitSignal(t, shutdownStarted)
	select {
	case err := <-done:
		t.Fatalf("shutdown returned before the request finished: %v", err)
	default:
	}
	finish()
	if err := awaitResult(t, requestDone); err != nil {
		t.Fatalf("active request failed: %v", err)
	}
	if err := awaitResult(t, done); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestShutdownTimeoutClosesActiveConnections(t *testing.T) {
	a := newTestApp(20 * time.Millisecond)
	started := make(chan struct{})
	requestCanceled := make(chan struct{})
	a.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(requestCanceled)
	})
	url, cancel, done := startTestApp(t, a)
	client := newTestClient(t)
	requestDone := make(chan error, 1)
	go func() {
		resp, err := client.Get(url)
		if resp != nil {
			resp.Body.Close()
		}
		requestDone <- err
	}()

	awaitSignal(t, started)
	cancel()
	if err := awaitResult(t, done); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected shutdown deadline error, got %v", err)
	}
	awaitSignal(t, requestCanceled)
	if err := awaitResult(t, requestDone); err == nil {
		t.Fatal("expected active connection to be closed")
	}
}

func TestRunReportsBusyAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	a := newTestApp(time.Second)
	a.server.Addr = listener.Addr().String()
	if err := a.Run(t.Context()); err == nil {
		t.Fatal("expected an error when the address is in use")
	}
}

func TestRunWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := newTestApp(time.Second).Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func newTestApp(timeout time.Duration) *App {
	return New(config.Config{
		HTTPAddr:        "127.0.0.1:0",
		ShutdownTimeout: timeout,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func startTestApp(t *testing.T, a *App) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- a.serve(ctx, listener)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		awaitResult(t, done)
	})
	return "http://" + listener.Addr().String(), cancel, done
}

func newTestClient(t *testing.T) *http.Client {
	t.Helper()
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 3 * time.Second}
}

func awaitResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for completion")
		return nil
	}
}

func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for signal")
	}
}
