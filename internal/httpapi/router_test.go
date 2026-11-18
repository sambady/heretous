package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sambady/heretous/internal/httpapi"
	"github.com/sambady/heretous/internal/testutil"
)

func TestReadinessTracksDatabaseAvailability(t *testing.T) {
	db, _ := testutil.Database(t)
	handler := httpapi.NewHandler(db)
	check := func(path string, status int) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != status {
			t.Fatalf("%s status=%d, want %d", path, response.Code, status)
		}
	}
	check("/readyz", http.StatusOK)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	check("/readyz", http.StatusServiceUnavailable)
	check("/healthz", http.StatusOK)
}
