package service

import (
	"api/dto"
	"api/model"
	"api/repository"
	"errors"
	"gorm.io/gorm"
)

type IBenefitService interface {
	NewBenefit(request *dto.BenefitRequest) error
	DeleteBenefit(benefitId string) error
	GetBenefitsByProgram(programID string, request *dto.BenefitByProgramQueryParams) ([]dto.OlympiadBenefitTree, error)
	GetBenefitsByOlympiad(olympiadID string, request *dto.BenefitByOlympiadQueryParams) ([]dto.ProgramBenefitTree, error)
	GetBenefitsByDiploma(diplomaID string, request *dto.BenefitByOlympiadQueryParams) ([]dto.ProgramBenefitTree, error)
	GetUserBenefits(userID any, request *dto.BenefitByOlympiadQueryParams) ([]dto.ProgramBenefitTree, error)
}

type BenefitService struct {
	benefitRepo repository.IBenefitRepo
	diplomaRepo repository.IDiplomaRepo
}

func NewBenefitService(benefitRepo repository.IBenefitRepo, diplomaRepo repository.IDiplomaRepo) *BenefitService {
	return &BenefitService{benefitRepo: benefitRepo, diplomaRepo: diplomaRepo}
}

func (b *BenefitService) NewBenefit(request *dto.BenefitRequest) error {
	benefitModel := newBenefitModel(request)
	return b.benefitRepo.NewBenefit(benefitModel)
}

func (b *BenefitService) DeleteBenefit(benefitId string) error {
	return b.benefitRepo.DeleteBenefit(benefitId)
}

func (b *BenefitService) GetBenefitsByProgram(programID string, request *dto.BenefitByProgramQueryParams) ([]dto.OlympiadBenefitTree, error) {
	benefits, err := b.benefitRepo.GetBenefitsByProgram(programID, request)
	if err != nil {
		return nil, err
	}
	return newOlympiadBenefitTrees(benefits), nil
}

func (b *BenefitService) GetBenefitsByOlympiad(olympiadID string, request *dto.BenefitByOlympiadQueryParams) ([]dto.ProgramBenefitTree, error) {
	benefits, err := b.benefitRepo.GetBenefitsByOlympiad(olympiadID, request)
	if err != nil {
		return nil, err
	}
	return newProgramBenefitTrees(benefits), nil
}

func (b *BenefitService) GetBenefitsByDiploma(diplomaID string, request *dto.BenefitByOlympiadQueryParams) ([]dto.ProgramBenefitTree, error) {
	diploma, err := b.diplomaRepo.GetDiplomaByID(diplomaID)
	if err != nil {
		return nil, err
	}

	user, ok := request.UserID.(uint)
	if !ok || user == 0 || diploma.UserID != user {
		return nil, gorm.ErrRecordNotFound
	}
	benefits, err := b.benefitRepo.GetBenefitsByDiplomas([]model.Diploma{*diploma}, request)
	if err != nil {
		return nil, err
	}
	return newProgramBenefitTrees(benefits), nil
}

func (b *BenefitService) GetUserBenefits(userID any, request *dto.BenefitByOlympiadQueryParams) ([]dto.ProgramBenefitTree, error) {
	uintUserID, ok := userID.(uint)
	if !ok {
		return nil, errors.New("userID must be uint")
	}

	diplomas, err := b.diplomaRepo.GetDiplomasByUserID(uintUserID)
	if err != nil {
		return nil, err
	}

	benefits, err := b.benefitRepo.GetBenefitsByDiplomas(diplomas, request)
	if err != nil {
		return nil, err
	}
	return newProgramBenefitTrees(benefits), nil
}

func newBenefitModel(request *dto.BenefitRequest) *model.Benefit {
	benefit := model.Benefit{
		ProgramID:       request.ProgramID,
		OlympiadID:      request.OlympiadID,
		MinClass:        request.MinClass,
		MinDiplomaLevel: request.MinDiplomaLevel,
		BVI:             request.BVI,
		ConfSubjRel:     make([]model.ConfirmationSubjects, len(request.ConfirmSubjects)),
	}

	for i := range request.ConfirmSubjects {
		benefit.ConfSubjRel[i] = model.ConfirmationSubjects{
			SubjectID: request.ConfirmSubjects[i].SubjectID,
			Score:     request.ConfirmSubjects[i].Score,
		}
	}

	if !request.BVI {
		benefit.FullScoreSubjects = make([]model.Subject, len(request.FullScoreSubjects))
		for i := range request.FullScoreSubjects {
			benefit.FullScoreSubjects[i] = model.Subject{
				SubjectID: request.FullScoreSubjects[i],
			}
		}
	}

	return &benefit
}

func newOlympiadBenefitTrees(benefits []model.Benefit) []dto.OlympiadBenefitTree {
	result := make([]dto.OlympiadBenefitTree, 0)
	indexes := map[uint]int{}
	var currentTree *dto.OlympiadBenefitTree

	for _, b := range benefits {
		if index, ok := indexes[b.OlympiadID]; ok {
			currentTree = &result[index]
		} else {
			indexes[b.OlympiadID] = len(result)
			tree := dto.OlympiadBenefitTree{
				Olympiad: dto.OlympiadBenefitInfo{
					OlympiadID: b.Olympiad.OlympiadID,
					Name:       b.Olympiad.Name,
					Level:      b.Olympiad.Level,
					Profile:    b.Olympiad.Profile,
				},
			}
			result = append(result, tree)
			currentTree = &result[len(result)-1]
		}

		if currentTree == nil {
			continue
		}

		currentTree.Benefits = append(currentTree.Benefits, extractBenefitInfo(b))
	}
	return result
}

func newProgramBenefitTrees(benefits []model.Benefit) []dto.ProgramBenefitTree {
	result := make([]dto.ProgramBenefitTree, 0)
	indexes := map[uint]int{}
	var currentTree *dto.ProgramBenefitTree

	for _, b := range benefits {
		if index, ok := indexes[b.ProgramID]; ok {
			currentTree = &result[index]
		} else {
			indexes[b.ProgramID] = len(result)
			tree := dto.ProgramBenefitTree{
				Program: dto.ProgramBenefitInfo{
					ProgramID:    b.Program.ProgramID,
					Name:         b.Program.Name,
					Field:        b.Program.Field.Code,
					UniShortName: b.Program.University.ShortName,
					UniversityID: b.Program.UniversityID,
				},
			}
			result = append(result, tree)
			currentTree = &result[len(result)-1]
		}

		if currentTree == nil {
			continue
		}

		currentTree.Benefits = append(currentTree.Benefits, extractBenefitInfo(b))
	}

	return result
}

func extractBenefitInfo(b model.Benefit) dto.BenefitInfo {
	benefitInfo := dto.BenefitInfo{
		MinClass:        &b.MinClass,
		MinDiplomaLevel: &b.MinDiplomaLevel,
		BVI:             b.BVI,
	}

	if len(b.AdmissionRule) > 0 {
		benefitInfo.MinClass = nil
		benefitInfo.MinDiplomaLevel = nil
		benefitInfo.AdmissionRule = b.AdmissionRule
		benefitInfo.SourceRelation = b.SourceRelation
	}

	for i := range b.ConfirmationSubjects {
		if i < len(b.ConfSubjRel) {
			benefitInfo.ConfirmSubjects = append(benefitInfo.ConfirmSubjects, dto.ConfirmSubjectResp{
				Name:  b.ConfirmationSubjects[i].Name,
				Score: b.ConfSubjRel[i].Score,
			})
		}
	}

	for _, subj := range b.FullScoreSubjects {
		benefitInfo.FullScoreSubjects = append(benefitInfo.FullScoreSubjects, subj.Name)
	}

	return benefitInfo
}
