package dto

import "encoding/json"

type ProgramShortResponse struct {
	AdmissionMetadata json.RawMessage `json:"admission_metadata,omitempty"`
	ProgramID         uint            `json:"program_id"`
	Name              string          `json:"name"`
	Field             string          `json:"field"`
	BudgetPlaces      *uint           `json:"budget_places"`
	PaidPlaces        *uint           `json:"paid_places"`
	Cost              *uint           `json:"cost"`
	RequiredSubjects  []string        `json:"required_subjects"`
	OptionalSubjects  []string        `json:"optional_subjects"`
	Link              string          `json:"link"`
	Like              bool            `json:"like"`
}

type ProgramResponse struct {
	ProgramShortResponse
	University UniversityProgramInfo `json:"university"`
}

type ProgramRequest struct {
	Name             string `json:"name" binding:"required"`
	BudgetPlaces     *uint  `json:"budget_places"`
	PaidPlaces       *uint  `json:"paid_places"`
	Cost             *uint  `json:"cost"`
	Link             string `json:"link"`
	UniversityID     uint   `json:"university_id" binding:"required"`
	FacultyID        uint   `json:"faculty_id" binding:"required"`
	FieldID          uint   `json:"field_id" binding:"required"`
	OptionalSubjects []uint `json:"optional_subjects"`
	RequiredSubjects []uint `json:"required_subjects"`
}

type ProgramBenefitInfo struct {
	ProgramID    uint   `json:"program_id"`
	Name         string `json:"name"`
	Field        string `json:"field"`
	UniShortName string `json:"university"`
	UniversityID uint   `json:"university_id"`
}

type ProgramTreeQueryParams struct {
	Degrees  []string `form:"degree"`
	Subjects []string `form:"subject"`
	Search   string   `form:"search"`
}

type ProgramsByFieldQueryParams struct {
	University []string `form:"university"`
	Search     string   `form:"search"`
	Subjects   []string `form:"subject"`
}
