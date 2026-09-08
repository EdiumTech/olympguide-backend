package handler

import (
	"api/utils/constants"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func personalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("OLYMPGUIDE_TEST_DSN")
	if dsn == "" {
		t.Skip("Set OLYMPGUIDE_TEST_DSN to a migrated disposable database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback(); sqlDB, _ := db.DB(); sqlDB.Close() })
	return tx
}
func testUser(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	var id uint
	if err := db.Raw(`INSERT INTO olympguide."user"(email,password_hash) VALUES(?,?) RETURNING user_id`, uuid.NewString()+"@test.invalid", "not-a-real-password").Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	return id
}
func calendarRouter(db *gorm.DB, user uint) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user != 0 {
			c.Set(constants.ContextUserID, user)
		}
	})
	h := NewCalendarHandler(db)
	r.GET("/plan", h.Plan)
	r.POST("/plan", h.Add)
	r.POST("/custom", h.Custom)
	r.PUT("/plan/:id", h.Update)
	r.DELETE("/plan/:id", h.Delete)
	r.GET("/events", h.Events)
	return r
}
func requestJSON(t *testing.T, r *gin.Engine, method, path, body string, status int) []byte {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("%s %s = %d, wanted %d: %s", method, path, w.Code, status, w.Body.String())
	}
	return w.Body.Bytes()
}
func TestCalendarAuthenticationBeforeDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := calendarRouter(nil, 0)
	for _, p := range []struct{ method, path string }{{"GET", "/plan"}, {"POST", "/plan"}, {"POST", "/custom"}, {"PUT", "/plan/1"}, {"DELETE", "/plan/1"}} {
		requestJSON(t, r, p.method, p.path, `{}`, 401)
	}
	r = calendarRouter(nil, 1)
	requestJSON(t, r, "POST", "/plan", `{"event_id":"x","user_id":2}`, 400)
}
func TestCalendarOwnershipReimportAndPersonalMarks(t *testing.T) {
	db := personalTestDB(t)
	a, b := testUser(t, db), testUser(t, db)
	r := calendarRouter(db, a)
	other := calendarRouter(db, b)
	key := "test-" + uuid.NewString()
	event := `{"title":"Registration","kind":"registration_close","time_kind":"timed","timezone":"Europe/Moscow","starts_at":"2026-09-21T14:00:00+03:00","description":"source","season":"2026/2027"}`
	if err := db.Exec("INSERT INTO olympguide.calendar_event(id,season,collection_key,revision,payload) VALUES(?,'2026/2027','test','r1',?::jsonb)", key, event).Error; err != nil {
		t.Fatal(err)
	}
	first := requestJSON(t, r, "POST", "/plan", `{"event_id":"`+key+`"}`, 200)
	if string(first) != string(requestJSON(t, r, "POST", "/plan", `{"event_id":"`+key+`"}`, 200)) {
		t.Fatal("duplicate add changed identity")
	}
	var result struct {
		PlanID json.Number `json:"plan_id"`
	}
	json.Unmarshal(first, &result)
	path := "/plan/" + result.PlanID.String()
	requestJSON(t, r, "PUT", path, `{"status":"registered","note":"keep me"}`, 200)
	requestJSON(t, other, "PUT", path, `{"status":"skipped","note":"overwrite"}`, 404)
	requestJSON(t, other, "DELETE", path, `{}`, 404)
	if string(requestJSON(t, other, "GET", "/plan", "", 200)) != "[]" {
		t.Fatal("another user's events leaked")
	}
	if err := db.Exec("UPDATE olympguide.calendar_event SET revision='r2',payload=jsonb_set(payload,'{starts_at}','\"2026-09-22T14:00:00+03:00\"') WHERE id=?", key).Error; err != nil {
		t.Fatal(err)
	}
	requestJSON(t, r, "POST", "/plan", `{"event_id":"`+key+`"}`, 200)
	var rows []planRow
	json.Unmarshal(requestJSON(t, r, "GET", "/plan", "", 200), &rows)
	if len(rows) != 1 || rows[0].Status != "registered" || rows[0].Note != "keep me" || !rows[0].DatesChanged || !strings.Contains(string(rows[0].Event), "2026-09-22") {
		t.Fatalf("marks lost or stale event: %+v", rows)
	}
	requestJSON(t, r, "PUT", path, `{"status":"completed","note":"keep me","acknowledge":true}`, 200)
	json.Unmarshal(requestJSON(t, r, "GET", "/plan", "", 200), &rows)
	if rows[0].DatesChanged {
		t.Fatal("acknowledgement not persisted")
	}
	db.Exec("UPDATE olympguide.calendar_event SET active=false WHERE id=?", key)
	json.Unmarshal(requestJSON(t, r, "GET", "/plan", "", 200), &rows)
	if rows[0].SourceActive {
		t.Fatal("withdrawn event lost")
	}
	requestJSON(t, r, "DELETE", path, `{}`, 200)
}
func TestCustomCalendarIdempotencyAndInvalidDates(t *testing.T) {
	db := personalTestDB(t)
	r := calendarRouter(db, testUser(t, db))
	key := uuid.NewString()
	body := `{"client_key":"` + key + `","event":{"title":"My event","kind":"custom","time_kind":"date_range","timezone":"Europe/Moscow","start_date":"2026-12-31","end_date":"2027-01-02","description":"","season":""}}`
	first := requestJSON(t, r, "POST", "/custom", body, 200)
	if string(first) != string(requestJSON(t, r, "POST", "/custom", body, 200)) {
		t.Fatal("custom retry duplicated event")
	}
	requestJSON(t, r, "POST", "/custom", strings.Replace(body, "2027-01-02", "2026-01-02", 1), 400)
	requestJSON(t, r, "POST", "/custom", strings.Replace(body, "My event", "Changed after uncertain response", 1), 409)
}
