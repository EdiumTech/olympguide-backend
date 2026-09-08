package router

import (
	"api/handler"
	"api/middleware"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions/cookie"
)

func testRouter() *Router {
	return NewRouter(&handler.Handlers{}, middleware.NewMw(nil, nil), cookie.NewStore([]byte("test-only-signing-secret-32-bytes!")))
}

func TestAnonymousProgramCreationIsRejected(t *testing.T) {
	t.Setenv("BEARER_DATA_LOADER_TOKEN", "")
	router := testRouter()
	for _, authorization := range []string{"", "Bearer ", "Bearer wrong"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/program/", nil)
		req.Header.Set("Authorization", authorization)
		recorder := httptest.NewRecorder()
		router.engine.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous program creation returned %d", recorder.Code)
		}
	}
}

func TestReadinessReportsDependencyFailure(t *testing.T) {
	for _, available := range []bool{true, false} {
		router := testRouter()
		router.RegisterHealth(func(context.Context) error {
			if !available {
				return errors.New("database password must never be exposed")
			}
			return nil
		})
		recorder := httptest.NewRecorder()
		router.engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		expected := http.StatusServiceUnavailable
		if available {
			expected = http.StatusOK
		}
		if recorder.Code != expected {
			t.Fatalf("readiness status %d, expected %d", recorder.Code, expected)
		}
		if !available && recorder.Body.String() != `{"status":"unavailable"}` {
			t.Fatalf("dependency details leaked: %s", recorder.Body.String())
		}
	}
}
