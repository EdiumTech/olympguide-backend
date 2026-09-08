package dto

type FacultyNewRequest struct {
	FacultyUpdateRequest
	UniversityID uint `json:"university_id" binding:"required"`
}

type FacultyUpdateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type FacultyShortResponse struct {
	ParentID  *uint  `json:"parent_id"`
	UnitType  string `json:"unit_type"`
	FacultyID uint   `json:"faculty_id"`
	Name      string `json:"name"`
}

type FacultyProgramTree struct {
	FacultyID uint                   `json:"faculty_id"`
	Name      string                 `json:"name"`
	Programs  []ProgramShortResponse `json:"programs"`
}
