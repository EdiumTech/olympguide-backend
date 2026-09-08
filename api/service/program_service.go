package service

import (
	"api/dto"
	"api/model"
	"api/repository"
	"api/utils/constants"
	"api/utils/errs"
)

type IProgramService interface {
	GetProgramsByFieldID(fieldID string, userID any, params *dto.ProgramsByFieldQueryParams) ([]dto.UniverProgramTree, error)
	GetProgramsByFacultyID(facultyID string, userID any) ([]dto.ProgramShortResponse, error)
	GetUniverProgramsByFaculty(univerID string, userID any, params *dto.ProgramTreeQueryParams) ([]dto.FacultyProgramTree, error)
	GetUniverProgramsByField(univerID string, userID any, params *dto.ProgramTreeQueryParams) ([]dto.GroupProgramTree, error)
	LikeProgram(programID string, userID uint) (bool, error)
	DislikeProgram(programID string, userID uint) (bool, error)
	GetLikedPrograms(userID uint) ([]dto.UniverProgramTree, error)
	GetProgram(programID string, userID any) (*dto.ProgramResponse, error)
	NewProgram(request *dto.ProgramRequest) (uint, error)
}

type ProgramService struct {
	programRepo repository.IProgramRepo
	univerRepo  repository.IUniverRepo
	facultyRepo repository.IFacultyRepo
	fieldRepo   repository.IFieldRepo
}

func NewProgramService(programRepo repository.IProgramRepo,
	univerRepo repository.IUniverRepo,
	facultyRepo repository.IFacultyRepo,
	fieldRepo repository.IFieldRepo) *ProgramService {
	return &ProgramService{programRepo: programRepo, univerRepo: univerRepo, facultyRepo: facultyRepo, fieldRepo: fieldRepo}
}

func (p *ProgramService) GetProgram(programID string, userID any) (*dto.ProgramResponse, error) {
	program, err := p.programRepo.GetProgram(programID, userID)
	if err != nil {
		return nil, err
	}
	return newProgramResponse(program), nil
}

func (p *ProgramService) GetLikedPrograms(userID uint) ([]dto.UniverProgramTree, error) {
	programs, err := p.programRepo.GetLikedPrograms(userID)
	if err != nil {
		return nil, err
	}
	return newUniverProgramTree(programs), nil
}

func (p *ProgramService) NewProgram(request *dto.ProgramRequest) (uint, error) {
	program := newProgramModel(request)

	if !p.facultyRepo.ExistsInUniversity(program.FacultyID, program.UniversityID) {
		return 0, errs.FacultyNotInUniversity
	}

	return p.programRepo.NewProgram(program)
}

func (p *ProgramService) LikeProgram(programID string, userID uint) (bool, error) {
	program, err := p.programRepo.GetProgram(programID, userID)
	if err != nil {
		return false, err
	}

	if program.Like {
		return false, nil
	}

	err = p.programRepo.LikeProgram(program.ProgramID, userID)
	if err != nil {
		return false, err
	}
	p.programRepo.ChangeProgramPopularity(program, constants.LikeProgramPopularIncr)
	return true, nil
}

func (p *ProgramService) DislikeProgram(programID string, userID uint) (bool, error) {
	program, err := p.programRepo.GetProgram(programID, userID)
	if err != nil {
		return false, err
	}

	if !program.Like {
		return false, nil
	}

	err = p.programRepo.DislikeProgram(program.ProgramID, userID)
	if err != nil {
		return false, err
	}
	p.programRepo.ChangeProgramPopularity(program, constants.LikeProgramPopularDecr)
	return true, nil
}

func (p *ProgramService) GetProgramsByFacultyID(facultyID string, userID any) ([]dto.ProgramShortResponse, error) {
	programs, err := p.programRepo.GetProgramsByFacultyID(facultyID, userID)
	if err != nil {
		return nil, err
	}
	response := make([]dto.ProgramShortResponse, len(programs))
	for i, program := range programs {
		response[i] = *newProgramShortResponse(&program)
	}
	return response, nil
}

func (p *ProgramService) GetProgramsByFieldID(fieldID string, userID any, params *dto.ProgramsByFieldQueryParams) ([]dto.UniverProgramTree, error) {
	programs, err := p.programRepo.GetProgramsByFieldID(fieldID, userID, params)
	if err != nil {
		return nil, err
	}
	return newUniverProgramTree(programs), nil
}

func (p *ProgramService) GetUniverProgramsByFaculty(univerID string, userID any, params *dto.ProgramTreeQueryParams) ([]dto.FacultyProgramTree, error) {
	programs, err := p.programRepo.GetUniverProgramsWithFaculty(univerID, userID, params)
	if err != nil {
		return nil, err
	}
	return newFacultyProgramTree(programs), nil
}

func (p *ProgramService) GetUniverProgramsByField(univerID string, userID any, params *dto.ProgramTreeQueryParams) ([]dto.GroupProgramTree, error) {
	programs, err := p.programRepo.GetUniverProgramsWithGroup(univerID, userID, params)
	if err != nil {
		return nil, err
	}
	return newGroupProgramTree(programs), nil
}

func newProgramShortResponse(program *model.Program) *dto.ProgramShortResponse {
	requiredSubjects := make([]string, len(program.RequiredSubjects))
	for i, s := range program.RequiredSubjects {
		requiredSubjects[i] = s.Name
	}
	optionalSubjects := make([]string, len(program.OptionalSubjects))
	for i, s := range program.OptionalSubjects {
		optionalSubjects[i] = s.Name
	}

	return &dto.ProgramShortResponse{
		ProgramID:         program.ProgramID,
		AdmissionMetadata: program.AdmissionMetadata,
		Name:              program.Name,
		Field:             program.Field.Code,
		BudgetPlaces:      program.BudgetPlaces,
		PaidPlaces:        program.PaidPlaces,
		Cost:              program.Cost,
		Link:              program.Link,
		Like:              program.Like,
		RequiredSubjects:  requiredSubjects,
		OptionalSubjects:  optionalSubjects,
	}
}

func newProgramResponse(program *model.Program) *dto.ProgramResponse {
	requiredSubjects := make([]string, len(program.RequiredSubjects))
	for i, s := range program.RequiredSubjects {
		requiredSubjects[i] = s.Name
	}

	optionalSubjects := make([]string, len(program.OptionalSubjects))
	for i, s := range program.OptionalSubjects {
		optionalSubjects[i] = s.Name
	}

	return &dto.ProgramResponse{
		ProgramShortResponse: *newProgramShortResponse(program),
		University: dto.UniversityProgramInfo{
			UniversityID: program.UniversityID,
			Name:         program.University.Name,
			ShortName:    program.University.ShortName,
			Region:       program.University.Region.Name,
			Logo:         program.University.Logo,
		},
	}
}

func newProgramModel(request *dto.ProgramRequest) *model.Program {
	program := model.Program{
		Name:             request.Name,
		BudgetPlaces:     request.BudgetPlaces,
		PaidPlaces:       request.PaidPlaces,
		Cost:             request.Cost,
		Link:             request.Link,
		UniversityID:     request.UniversityID,
		FieldID:          request.FieldID,
		FacultyID:        request.FacultyID,
		OptionalSubjects: make([]model.Subject, len(request.OptionalSubjects)),
		RequiredSubjects: make([]model.Subject, len(request.RequiredSubjects)),
	}

	for i := range request.OptionalSubjects {
		program.OptionalSubjects[i] = model.Subject{
			SubjectID: request.OptionalSubjects[i],
		}
	}

	for i := range request.RequiredSubjects {
		program.RequiredSubjects[i] = model.Subject{
			SubjectID: request.RequiredSubjects[i],
		}
	}
	return &program
}

func newFacultyProgramTree(programs []model.Program) []dto.FacultyProgramTree {
	result := make([]dto.FacultyProgramTree, 0)
	indexes := map[uint]int{}
	for _, p := range programs {
		faculties := p.Faculties
		if len(faculties) == 0 {
			faculties = []model.Faculty{p.Faculty}
		}
		for _, f := range faculties {
			if f.ParentID != nil {
				continue
			}
			name := f.Name
			if f.FacultyID == 0 {
				name = "Подразделение не подтверждено"
			}
			index, ok := indexes[f.FacultyID]
			if !ok {
				index = len(result)
				indexes[f.FacultyID] = index
				result = append(result, dto.FacultyProgramTree{FacultyID: f.FacultyID, Name: name, Programs: make([]dto.ProgramShortResponse, 0)})
			}
			result[index].Programs = append(result[index].Programs, *newProgramShortResponse(&p))
		}
	}
	return result
}

func newGroupProgramTree(programs []model.Program) []dto.GroupProgramTree {
	var result []dto.GroupProgramTree
	var currentTree *dto.GroupProgramTree
	var currentGroupID uint

	for _, program := range programs {
		if program.Field.GroupID != currentGroupID {
			currentGroupID = program.Field.GroupID
			tree := dto.GroupProgramTree{
				GroupID: program.Field.GroupID,
				Name:    program.Field.Group.Name,
				Code:    program.Field.Group.Code,
			}
			result = append(result, tree)
			currentTree = &result[len(result)-1]
		}

		if currentTree == nil {
			continue
		}

		currentTree.Programs = append(currentTree.Programs, *newProgramShortResponse(&program))
	}

	return result
}

func newUniverProgramTree(programs []model.Program) []dto.UniverProgramTree {
	var result []dto.UniverProgramTree
	var currentTree *dto.UniverProgramTree
	var currentUniverID uint

	for _, program := range programs {
		if program.UniversityID != currentUniverID {
			currentUniverID = program.UniversityID
			tree := dto.UniverProgramTree{
				Univer: dto.UniversityProgramInfo{
					UniversityID: program.UniversityID,
					Name:         program.University.Name,
					ShortName:    program.University.ShortName,
					Region:       program.University.Region.Name,
					Logo:         program.University.Logo,
				},
			}
			result = append(result, tree)
			currentTree = &result[len(result)-1]
		}

		if currentTree == nil {
			continue
		}

		currentTree.Programs = append(currentTree.Programs, *newProgramShortResponse(&program))
	}

	return result
}
