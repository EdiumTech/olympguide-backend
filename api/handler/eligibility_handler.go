package handler

import (
	"api/dto"
	"api/service"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sort"
	"strconv"
	"strings"
)

type EligibilityHandler struct{ db *gorm.DB }

func NewEligibilityHandler(db *gorm.DB) *EligibilityHandler { return &EligibilityHandler{db} }

func (h *EligibilityHandler) UpdateDiploma(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	id, ok := planID(c)
	if !ok {
		return
	}
	var body struct {
		dto.DiplomaDetails
		Class      uint  `json:"class"`
		Level      uint  `json:"level"`
		OlympiadID *uint `json:"olympiad_id,omitempty"`
	}
	if !personalJSON(c, &body) {
		return
	}
	if err := service.ValidateDiplomaDetails(body.DiplomaDetails, body.Class, body.Level); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	updates := map[string]any{"class": body.Class, "level": body.Level, "award_year": body.AwardYear, "olympiad_year": body.OlympiadYear, "profile": body.Profile, "result": body.Result}
	if body.OlympiadID != nil {
		var n int64
		if err := h.db.Table("olympguide.olympiad").Where("olympiad_id=?", *body.OlympiadID).Count(&n).Error; err != nil {
			calendarError(c, err)
			return
		}
		if n == 0 {
			c.JSON(400, gin.H{"message": "Олимпиада не найдена"})
			return
		}
		updates["olympiad_id"] = *body.OlympiadID
	}
	r := h.db.WithContext(c.Request.Context()).Table("olympguide.diploma").Where("user_id=? AND diploma_id=?", user, id).Updates(updates)
	if r.Error != nil {
		calendarError(c, r.Error)
		return
	}
	if r.RowsAffected == 0 {
		c.JSON(404, gin.H{"message": "Диплом не найден"})
		return
	}
	c.JSON(200, gin.H{"message": "Сохранено"})
}

func (h *EligibilityHandler) Catalog(c *gin.Context) {
	var olympiads = []struct {
		OlympiadID     uint    `json:"olympiad_id"`
		CatalogKey     string  `json:"catalog_key"`
		AcademicYear   *string `json:"academic_year,omitempty"`
		RegistryStatus *string `json:"registry_status,omitempty"`
		AdmissionYear  *uint   `json:"admission_year,omitempty"`
		Name           string  `json:"name"`
		Profile        string  `json:"profile"`
		Category       string  `json:"category"`
	}{}
	var programs = []struct {
		ProgramID     uint   `json:"program_id"`
		CatalogKey    string `json:"catalog_key"`
		Name          string `json:"name"`
		UniversityKey string `json:"university_key"`
	}{}
	db := h.db.WithContext(c.Request.Context())
	if err := db.Table("olympguide.olympiad").Select("olympiad_id,catalog_key,name,profile,category,academic_year,registry_status,admission_year").Where("catalog_key IS NOT NULL").Order("name,profile,olympiad_id").Scan(&olympiads).Error; err != nil {
		calendarError(c, err)
		return
	}
	if err := db.Table("olympguide.educational_program p").Select("p.program_id,p.catalog_key,p.name,u.catalog_key AS university_key").Joins("JOIN olympguide.university u USING(university_id)").Where("p.catalog_key IS NOT NULL").Order("p.program_id").Scan(&programs).Error; err != nil {
		calendarError(c, err)
		return
	}
	c.JSON(200, gin.H{"olympiads": olympiads, "programs": programs})
}
func (h *EligibilityHandler) Profile(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	var row struct{ Payload json.RawMessage }
	if err := h.db.WithContext(c.Request.Context()).Table("olympguide.user_admission_profile").Where("user_id=?", user).Scan(&row).Error; err != nil {
		calendarError(c, err)
		return
	}
	if len(row.Payload) == 0 {
		c.JSON(200, service.ApplicantProfile{Exams: []service.ExamResult{}})
		return
	}
	c.Data(200, "application/json", row.Payload)
}
func (h *EligibilityHandler) SaveProfile(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	var p service.ApplicantProfile
	if !personalJSON(c, &p) {
		return
	}
	if err := service.ValidateApplicantProfile(p); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if p.Exams == nil {
		p.Exams = []service.ExamResult{}
	}
	payload, _ := json.Marshal(p)
	if err := h.db.WithContext(c.Request.Context()).Exec(`INSERT INTO olympguide.user_admission_profile(user_id,payload) VALUES(?,?::jsonb) ON CONFLICT(user_id) DO UPDATE SET payload=EXCLUDED.payload,updated_at=now()`, user, string(payload)).Error; err != nil {
		calendarError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Сохранено"})
}

type recommendationAssessment struct {
	service.Assessment
	DiplomaID  uint            `json:"diploma_id"`
	RuleID     string          `json:"rule_id"`
	SourceRule json.RawMessage `json:"source_rule"`
}
type recommendation struct {
	ProgramID     uint                       `json:"program_id"`
	ProgramKey    string                     `json:"program_key"`
	Name          string                     `json:"name"`
	University    string                     `json:"university"`
	UniversityKey string                     `json:"university_key"`
	Status        string                     `json:"status"`
	Assessments   []recommendationAssessment `json:"assessments"`
}

func statusRank(s string) int {
	switch s {
	case service.Eligible:
		return 0
	case service.ConfirmationRequired:
		return 1
	case service.ManualReview:
		return 2
	default:
		return 3
	}
}

func (h *EligibilityHandler) Recommendations(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, err2 := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || err2 != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(400, gin.H{"message": "Некорректная пагинация"})
		return
	}
	status := c.Query("status")
	if status != "" && status != service.Eligible && status != service.ConfirmationRequired && status != service.Ineligible && status != service.ManualReview {
		c.JSON(400, gin.H{"message": "Неизвестный статус"})
		return
	}
	db := h.db.WithContext(c.Request.Context())
	diplomaID := uint64(0)
	if v := c.Query("diploma_id"); v != "" {
		diplomaID, err = strconv.ParseUint(v, 10, 32)
		if err != nil || diplomaID == 0 {
			c.JSON(400, gin.H{"message": "Некорректный диплом"})
			return
		}
		var count int64
		if err := db.Table("olympguide.diploma").Where("user_id=? AND diploma_id=?", user, diplomaID).Count(&count).Error; err != nil {
			calendarError(c, err)
			return
		}
		if count == 0 {
			c.JSON(404, gin.H{"message": "Диплом не найден"})
			return
		}
	}
	var stored struct{ Payload json.RawMessage }
	if err := db.Table("olympguide.user_admission_profile").Where("user_id=?", user).Scan(&stored).Error; err != nil {
		calendarError(c, err)
		return
	}
	p := service.ApplicantProfile{}
	if len(stored.Payload) > 0 {
		if err := json.Unmarshal(stored.Payload, &p); err != nil {
			calendarError(c, err)
			return
		}
	}
	empty := func(message string) {
		c.JSON(200, gin.H{"admission_year": p.AdmissionYear, "total": 0, "items": []recommendation{}, "message": message})
	}
	if p.AdmissionYear == nil {
		empty("Укажите год поступления и подтверждающие результаты ЕГЭ")
		return
	}
	var yearCount int64
	if err := db.Table("olympguide.admission_release").Where("admission_year=?", *p.AdmissionYear).Count(&yearCount).Error; err != nil {
		calendarError(c, err)
		return
	}
	if yearCount == 0 {
		empty(fmt.Sprintf("Правила приёма %d пока не подтверждены. Правила других лет не применяются.", *p.AdmissionYear))
		return
	}
	var rows []struct {
		DiplomaID       uint
		AwardYear       *int
		OlympiadYear    *string
		Profile         *string
		Class           uint
		Result          *string
		RuleID          string
		SourceRule      json.RawMessage
		Evaluation      json.RawMessage
		SourceUnchanged bool
		ProgramID       uint
		ProgramKey      string
		Name            string
		University      string
		UniversityKey   string
	}
	q := db.Table("olympguide.diploma d").Select(`d.diploma_id,d.award_year,d.olympiad_year,d.profile,d.class,d.result,ar.id AS rule_id,ar.payload AS source_rule,
        ae.payload AS evaluation,COALESCE(ae.source_payload=ar.payload,false) AS source_unchanged,
        p.program_id,p.catalog_key AS program_key,p.name,u.short_name AS university,u.catalog_key AS university_key`).
		Joins("JOIN olympguide.admission_rule ar ON ar.olympiad_id=d.olympiad_id AND ar.admission_year=?", *p.AdmissionYear).
		Joins("LEFT JOIN olympguide.admission_evaluation ae ON ae.admission_year=ar.admission_year AND ae.rule_id=ar.id").
		Joins("JOIN olympguide.educational_program p ON p.university_id=ar.university_id AND p.catalog_key IS NOT NULL AND (ae.id IS NULL OR jsonb_exists(ae.payload->'program_keys',p.catalog_key))").
		Joins("JOIN olympguide.university u ON u.university_id=p.university_id").Where("d.user_id=?", user)
	if diplomaID != 0 {
		q = q.Where("d.diploma_id=?", diplomaID)
	}
	if err := q.Order("p.program_id,d.diploma_id,ar.id,ae.id").Scan(&rows).Error; err != nil {
		calendarError(c, err)
		return
	}
	programs := map[uint]*recommendation{}
	for _, row := range rows {
		rule := service.EligibilityRule{}
		if len(row.Evaluation) > 0 {
			if err := json.Unmarshal(row.Evaluation, &rule); err != nil {
				calendarError(c, err)
				return
			}
		}
		if len(row.Evaluation) == 0 {
			rule.AdmissionYear = *p.AdmissionYear
			rule.ManualReason = "Правило ещё не разобрано для персональной проверки"
		}
		if b := c.Query("benefit"); b != "" && rule.Benefit != b {
			continue
		}
		assessment := service.EvaluateEligibility(rule, service.DiplomaFacts{AwardYear: row.AwardYear, OlympiadYear: row.OlympiadYear, Profile: row.Profile, Class: row.Class, Result: row.Result}, p, row.ProgramKey, row.SourceUnchanged)
		entry := programs[row.ProgramID]
		if entry == nil {
			entry = &recommendation{ProgramID: row.ProgramID, ProgramKey: row.ProgramKey, Name: row.Name, University: row.University, UniversityKey: row.UniversityKey, Status: assessment.Status, Assessments: []recommendationAssessment{}}
			programs[row.ProgramID] = entry
		}
		if statusRank(assessment.Status) < statusRank(entry.Status) {
			entry.Status = assessment.Status
		}
		entry.Assessments = append(entry.Assessments, recommendationAssessment{Assessment: assessment, DiplomaID: row.DiplomaID, RuleID: row.RuleID, SourceRule: row.SourceRule})
	}
	result := []recommendation{}
	text := strings.ToLower(c.Query("q"))
	for _, r := range programs {
		if (status == "" || r.Status == status) && (text == "" || strings.Contains(strings.ToLower(r.Name+" "+r.University), text)) {
			result = append(result, *r)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if statusRank(result[i].Status) != statusRank(result[j].Status) {
			return statusRank(result[i].Status) < statusRank(result[j].Status)
		}
		return result[i].ProgramID < result[j].ProgramID
	})
	total := len(result)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	message := "Соответствие условиям льготы не означает гарантированное зачисление. Проверка основана на введённых вами сведениях; документы проверяет вуз."
	if total == 0 {
		message = "Нет проверенных связей для выбранных дипломов и фильтров. Для старого диплома может потребоваться уточнение олимпиады и профиля; это не отказ в льготе."
	}
	c.JSON(200, gin.H{"admission_year": p.AdmissionYear, "total": total, "limit": limit, "offset": offset, "items": result[offset:end], "message": message})
}
