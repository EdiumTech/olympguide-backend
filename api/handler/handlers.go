package handler

type Handlers struct {
	Eligibility *EligibilityHandler
	Calendar    *CalendarHandler
	Scholarship *ScholarshipHandler
	Admission   *AdmissionHandler
	Auth        *AuthHandler
	Univer      *UniverHandler
	Field       *FieldHandler
	Olymp       *OlympHandler
	Meta        *MetaHandler
	User        *UserHandler
	Faculty     *FacultyHandler
	Program     *ProgramHandler
	Diploma     *DiplomaHandler
	Benefit     *BenefitHandler
}
