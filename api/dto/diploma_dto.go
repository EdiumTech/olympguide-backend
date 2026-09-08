package dto

type DiplomaRequest struct {
	UserID uint `json:"user_id" binding:"required"`
	DiplomaUserRequest
}

type DiplomaDetails struct {
	AwardYear    *int    `json:"award_year"`
	OlympiadYear *string `json:"olympiad_year"`
	Profile      *string `json:"profile"`
	Result       *string `json:"result"`
}

type DiplomaUserRequest struct {
	DiplomaDetails
	OlympiadID uint `json:"olympiad_id" binding:"required"`
	Class      uint `json:"class" binding:"required,min=1,max=11"`
	Level      uint `json:"level" binding:"required,min=1,max=3"`
}

type OlympDiplomaInfo struct {
	Name    string `json:"name"`
	Profile string `json:"profile"`
	Level   int16  `json:"level"`
}

type DiplomaResponse struct {
	DiplomaDetails
	OlympiadID uint             `json:"olympiad_id"`
	DiplomaID  uint             `json:"diploma_id"`
	Class      uint             `json:"class"`
	Level      uint             `json:"level"`
	Olympiad   OlympDiplomaInfo `json:"olympiad"`
}

type UploadDiplomasMessage struct {
	UserID     uint   `json:"user_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	SecondName string `json:"second_name"`
	Birthday   string `json:"birthday"`
}
