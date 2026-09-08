package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdmissionHandler exposes complete source rules, including scopes that cannot
// safely be reduced to a particular educational program or diploma match.
type AdmissionHandler struct{ db *gorm.DB }

func NewAdmissionHandler(db *gorm.DB) *AdmissionHandler { return &AdmissionHandler{db: db} }

func (h *AdmissionHandler) Status(c *gin.Context) {
	var rows []struct {
		Payload    json.RawMessage
		ImportedAt string
	}
	if err := h.db.WithContext(c.Request.Context()).Raw("SELECT payload, imported_at::text FROM olympguide.admission_release ORDER BY admission_year DESC").Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Cannot read admission catalogue"})
		return
	}
	result := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		result = append(result, gin.H{"catalogue": r.Payload, "imported_at": r.ImportedAt})
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdmissionHandler) Rules(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	year, yearErr := strconv.Atoi(c.DefaultQuery("admission_year", "2026"))
	if err != nil || offsetErr != nil || yearErr != nil || limit < 1 || limit > 100 || offset < 0 || year < 2000 || year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid pagination or admission year"})
		return
	}
	query := h.db.WithContext(c.Request.Context()).Table("olympguide.admission_rule AS ar").Where("ar.admission_year = ?", year)
	for _, key := range []string{"university_id", "olympiad_id"} {
		if value := c.Query(key); value != "" {
			id, err := strconv.ParseUint(value, 10, 32)
			if err != nil || id == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid entity ID"})
				return
			}
			query = query.Where("ar."+key+" = ?", id)
		}
	}
	if value := c.Query("program_id"); value != "" {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid program ID"})
			return
		}
		query = query.Where("EXISTS (SELECT 1 FROM olympguide.admission_program_rule pr WHERE pr.admission_year=ar.admission_year AND pr.rule_id=ar.id AND pr.program_id=?)", id)
	}
	if value := c.Query("q"); value != "" {
		query = query.Where("ar.payload::text ILIKE ?", "%"+value+"%")
	}
	if value := c.Query("category"); value != "" {
		query = query.Where("ar.category = ?", value)
	}
	if value := c.Query("benefit"); value != "" {
		query = query.Where("jsonb_exists(ar.payload->'benefit_types', ?)", value)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"message": "Cannot count admission rules"})
		return
	}
	var items []struct {
		ID           string          `json:"id"`
		UniversityID uint            `json:"university_id"`
		OlympiadID   uint            `json:"olympiad_id"`
		Payload      json.RawMessage `json:"rule"`
	}
	if err := query.Select("ar.id, ar.university_id, ar.olympiad_id, ar.payload").Order("ar.id").Limit(limit).Offset(offset).Scan(&items).Error; err != nil {
		c.JSON(500, gin.H{"message": "Cannot read admission rules"})
		return
	}
	if items == nil {
		items = make([]struct {
			ID           string          `json:"id"`
			UniversityID uint            `json:"university_id"`
			OlympiadID   uint            `json:"olympiad_id"`
			Payload      json.RawMessage `json:"rule"`
		}, 0)
	}
	c.JSON(http.StatusOK, gin.H{"admission_year": year, "total": total, "limit": limit, "offset": offset, "items": items})
}
