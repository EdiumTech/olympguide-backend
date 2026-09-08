package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
)

type ScholarshipHandler struct{ db *gorm.DB }

func NewScholarshipHandler(db *gorm.DB) *ScholarshipHandler { return &ScholarshipHandler{db} }

func (h *ScholarshipHandler) List(c *gin.Context) {
	ids := map[string]uint64{}
	for _, key := range []string{"university_id", "program_id"} {
		if value := c.Query(key); value != "" {
			id, err := strconv.ParseUint(value, 10, 32)
			if err != nil || id == 0 {
				c.JSON(400, gin.H{"message": "Invalid entity ID"})
				return
			}
			ids[key] = id
		}
	}
	q := h.db.WithContext(c.Request.Context()).Table("olympguide.scholarship AS s")
	if id := ids["university_id"]; id != 0 {
		q = q.Where("s.university_key=(SELECT catalog_key FROM olympguide.university WHERE university_id=?)", id)
	}
	if id := ids["program_id"]; id != 0 {
		// A faculty relationship is never evidence of a program-specific award.
		q = q.Where(`EXISTS (SELECT 1 FROM olympguide.educational_program p
            JOIN olympguide.university u ON u.university_id=p.university_id
            WHERE p.program_id=? AND u.catalog_key=s.university_key AND
            (s.payload->'scope'->>'type' = 'university' OR
             jsonb_exists(s.payload->'scope'->'program_keys',p.catalog_key)))`, id)
	}
	var rows []struct{ Payload json.RawMessage }
	if err := q.Select("s.payload").Order("s.id").Scan(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": "Cannot read scholarships"})
		return
	}
	result := make([]json.RawMessage, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.Payload)
	}
	c.JSON(200, result)
}
