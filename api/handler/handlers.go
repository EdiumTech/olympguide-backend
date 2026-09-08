package handler

type Handlers struct {
	Admission *AdmissionHandler
	Auth      *AuthHandler
	Univer    *UniverHandler
	Field     *FieldHandler
	Olymp     *OlympHandler
	Meta      *MetaHandler
	User      *UserHandler
	Faculty   *FacultyHandler
	Program   *ProgramHandler
	Diploma   *DiplomaHandler
	Benefit   *BenefitHandler
}
