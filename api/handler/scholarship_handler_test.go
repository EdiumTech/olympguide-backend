package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScholarshipsRejectInvalidIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/scholarships", NewScholarshipHandler(nil).List)
	for _, q := range []string{"program_id=0", "program_id=word", "university_id=-1", "program_id=4294967296"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/scholarships?"+q, nil))
		if w.Code != 400 {
			t.Fatalf("%s: %d", q, w.Code)
		}
	}
}

func TestScholarshipCampusDoesNotImplyProgramScope(t *testing.T) {
	db := personalTestDB(t)
	var program struct {
		ProgramID     uint
		UniversityKey string
		CatalogKey    string
	}
	if err := db.Raw(`SELECT p.program_id,p.catalog_key,u.catalog_key AS university_key FROM olympguide.educational_program p JOIN olympguide.university u USING(university_id) WHERE p.catalog_key IS NOT NULL LIMIT 1`).Scan(&program).Error; err != nil {
		t.Fatal(err)
	}
	if program.ProgramID == 0 {
		t.Fatal("import the base catalog")
	}
	for _, row := range []struct{ id, payload string }{
		{"test-campus-scope", `{"id":"test-campus-scope","scope":{"type":"campus","program_keys":[]}}`},
		{"test-explicit-scope", fmt.Sprintf(`{"id":"test-explicit-scope","scope":{"type":"program","program_keys":["%s"]}}`, program.CatalogKey)},
	} {
		if err := db.Exec(`INSERT INTO olympguide.scholarship(id,university_key,payload) VALUES(?,?,?::jsonb)`, row.id, program.UniversityKey, row.payload).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	r.GET("/scholarships", NewScholarshipHandler(db).List)
	body := string(requestJSON(t, r, "GET", fmt.Sprintf("/scholarships?program_id=%d", program.ProgramID), "", 200))
	if strings.Contains(body, "test-campus-scope") || !strings.Contains(body, "test-explicit-scope") {
		t.Fatal("unverified scope inherited", body)
	}
}
