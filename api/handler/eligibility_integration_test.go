package handler

import (
	"api/service"
	"api/utils/constants"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"testing"
)

func eligibilityRouter(db *gorm.DB, user uint) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user != 0 {
			c.Set(constants.ContextUserID, user)
		}
	})
	h := NewEligibilityHandler(db)
	r.GET("/profile", h.Profile)
	r.PUT("/profile", h.SaveProfile)
	r.GET("/matches", h.Recommendations)
	r.PUT("/diploma/:id", h.UpdateDiploma)
	return r
}
func TestEligibilityAuthenticationBeforeDB(t *testing.T) {
	r := eligibilityRouter(nil, 0)
	for _, test := range []struct{ method, path string }{{"GET", "/profile"}, {"PUT", "/profile"}, {"GET", "/matches"}, {"PUT", "/diploma/1"}} {
		requestJSON(t, r, test.method, test.path, "{}", 401)
	}
	r = eligibilityRouter(nil, 1)
	requestJSON(t, r, "PUT", "/profile", `{"admission_year":2026,"exams":[],"user_id":2}`, 400)
}
func TestRecommendationsOwnershipSourceChangeAndAdmissionYear(t *testing.T) {
	db := personalTestDB(t)
	a, b := testUser(t, db), testUser(t, db)
	r, other := eligibilityRouter(db, a), eligibilityRouter(db, b)
	var source struct {
		RuleID     string
		OlympiadID uint
		Profile    string
	}
	if err := db.Raw(`SELECT ar.id AS rule_id,ar.olympiad_id,ar.payload->'values'->>'olympiad_profile' AS profile
 FROM olympguide.admission_rule ar JOIN olympguide.admission_evaluation e ON e.rule_id=ar.id AND e.admission_year=ar.admission_year
 JOIN olympguide.university u ON u.university_id=ar.university_id WHERE u.catalog_key='msal' AND e.payload->>'complete'='true' AND e.payload->>'benefit'='bvi' ORDER BY ar.id LIMIT 1`).Scan(&source).Error; err != nil {
		t.Fatal(err)
	}
	if source.RuleID == "" {
		t.Fatal("Import the base and personal 2026 releases before integration tests")
	}
	var id uint
	if err := db.Raw(`INSERT INTO olympguide.diploma(user_id,olympiad_id,class,level,award_year,olympiad_year,profile,result) VALUES(?,?,11,2,2026,'2025/2026',?,'prize_winner') RETURNING diploma_id`, a, source.OlympiadID, source.Profile).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	requestJSON(t, r, "PUT", "/profile", `{"admission_year":2026,"exams":[{"subject":"обществознание","score":75,"year":2026}]}`, 200)
	if got := string(requestJSON(t, other, "GET", "/profile", "", 200)); got != `{"admission_year":null,"exams":[]}` {
		t.Fatal("profile leaked", got)
	}
	requestJSON(t, other, "GET", fmt.Sprintf("/matches?diploma_id=%d", id), "", 404)
	requestJSON(t, other, "PUT", fmt.Sprintf("/diploma/%d", id), `{"class":11,"level":1,"award_year":2025}`, 404)
	var response struct {
		Total   int
		Items   []recommendation
		Message string
	}
	json.Unmarshal(requestJSON(t, r, "GET", "/matches?status=eligible&limit=100", "", 200), &response)
	if response.Total == 0 {
		t.Fatalf("no eligible programs: %+v", response)
	}
	for _, p := range response.Items {
		if p.Status != service.Eligible {
			t.Fatal(p)
		}
		for _, assessment := range p.Assessments {
			if assessment.Status == service.Eligible && len(assessment.SourceRule) == 0 {
				t.Fatal("missing source")
			}
		}
	}
	// Same user data, changed source: parsed conditions must no longer pass.
	if err := db.Exec(`UPDATE olympguide.admission_rule SET payload=jsonb_set(payload,'{changed}','true') WHERE olympiad_id=?`, source.OlympiadID).Error; err != nil {
		t.Fatal(err)
	}
	response.Items = nil
	json.Unmarshal(requestJSON(t, r, "GET", "/matches?status=eligible", "", 200), &response)
	if response.Total != 0 {
		t.Fatal("stale source accepted")
	}
	requestJSON(t, r, "PUT", "/profile", `{"admission_year":2027,"exams":[]}`, 200)
	json.Unmarshal(requestJSON(t, r, "GET", "/matches", "", 200), &response)
	if response.Total != 0 || response.Message == "" {
		t.Fatal("2026 rules reused in 2027")
	}
	requestJSON(t, r, "PUT", fmt.Sprintf("/diploma/%d", id), `{"class":11,"level":2,"award_year":2026,"olympiad_year":"2024/2025"}`, 400)
}
