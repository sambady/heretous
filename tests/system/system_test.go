package system_test

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// These requests go to the real application started by Compose.
func TestRunningProduct(t *testing.T) {
	baseURL := os.Getenv("TEST_APP_URL")
	if baseURL == "" {
		t.Skip("TEST_APP_URL is not set; start the product and run the Compose tests service")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{"/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			response, err := client.Get(strings.TrimRight(baseURL, "/") + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || string(body) != "{\"status\":\"ok\"}\n" {
				t.Fatalf("unexpected response: status=%d body=%q", response.StatusCode, body)
			}
		})
	}
}
