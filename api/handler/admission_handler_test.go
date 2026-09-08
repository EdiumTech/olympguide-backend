package handler

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestAdmissionQueryRejectsInvalidPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAdmissionHandler(nil)
	r := gin.New()
	r.GET("/rules", h.Rules)
	// Invalid pagination must be rejected before any database access.
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "offset=-1", "admission_year=1"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/rules?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("%s: status %d", query, w.Code)
		}
	}
}
