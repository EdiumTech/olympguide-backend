package service

import (
	"api/dto"
	"api/model"
	pb "api/proto/gen"
	"api/repository"
	"api/utils/constants"
	"api/utils/errs"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"path/filepath"
)

type IUniverService interface {
	GetUniver(universityID string, userID any) (*dto.UniversityResponse, error)
	GetUnivers(params *dto.UniverBaseParams) ([]dto.UniversityShortResponse, error)
	GetBenefitByOlympUnivers(params *dto.UniverBaseParams, olympiadID string) ([]dto.UniversityShortResponse, error)
	GetDiplomaUnivers(params *dto.UniverBaseParams, diplomaID string) ([]dto.UniversityShortResponse, error)
	GetUserDiplomasUnivers(params *dto.UniverBaseParams) ([]dto.UniversityShortResponse, error)
	GetLikedUnivers(userID uint) ([]dto.UniversityShortResponse, error)
	NewUniver(request *dto.UniversityRequest) (uint, error)
	UpdateUniver(request *dto.UniversityRequest, universityID string) (uint, error)
	DeleteUniver(universityID string) error
	LikeUniver(universityID string, userID uint) (bool, error)
	DislikeUniver(universityID string, userID uint) (bool, error)
	UploadLogo(universityID string, file multipart.File, header *multipart.FileHeader) (*dto.UniverLogoResponse, error)
	DeleteLogo(universityID string) error
}

type UniverService struct {
	univerRepo           repository.IUniverRepo
	regionRepo           repository.IRegionRepo
	diplomaRepo          repository.IDiplomaRepo
	storageServiceClient pb.StorageServiceClient
}

func NewUniverService(
	univerRepo repository.IUniverRepo,
	regionRepo repository.IRegionRepo,
	diplomaRepo repository.IDiplomaRepo,
	storageServiceClient pb.StorageServiceClient) *UniverService {
	return &UniverService{
		univerRepo:           univerRepo,
		regionRepo:           regionRepo,
		diplomaRepo:          diplomaRepo,
		storageServiceClient: storageServiceClient,
	}
}

func (u *UniverService) GetUniver(universityID string, userID any) (*dto.UniversityResponse, error) {
	univer, err := u.univerRepo.GetUniver(universityID, userID)
	if err != nil {
		return nil, err
	}
	return newUniverResponse(univer), nil
}

func (u *UniverService) GetUnivers(params *dto.UniverBaseParams) ([]dto.UniversityShortResponse, error) {
	if uintUserID, ok := params.UserID.(uint); ok && params.FromMyRegion {
		region, err := u.regionRepo.GetUserRegion(uintUserID)
		if err != nil {
			return nil, err
		}
		params.Regions = []string{region.Name}
	}

	univers, err := u.univerRepo.GetUnivers(params)
	if err != nil {
		return nil, err
	}

	return newUniversShortResponse(univers), nil
}

func (u *UniverService) GetBenefitByOlympUnivers(params *dto.UniverBaseParams, olympiadID string) ([]dto.UniversityShortResponse, error) {
	if uintUserID, ok := params.UserID.(uint); ok && params.FromMyRegion {
		region, err := u.regionRepo.GetUserRegion(uintUserID)
		if err != nil {
			return nil, err
		}
		params.Regions = []string{region.Name}
	}

	univers, err := u.univerRepo.GetBenefitByOlympUnivers(params, olympiadID)
	if err != nil {
		return nil, err
	}
	return newUniversShortResponse(univers), nil
}

func (u *UniverService) GetDiplomaUnivers(params *dto.UniverBaseParams, diplomaID string) ([]dto.UniversityShortResponse, error) {
	diploma, err := u.diplomaRepo.GetDiplomaByID(diplomaID)
	if err != nil {
		return nil, err
	}

	if uintUserID, ok := params.UserID.(uint); ok && params.FromMyRegion {
		region, err := u.regionRepo.GetUserRegion(uintUserID)
		if err != nil {
			return nil, err
		}
		params.Regions = []string{region.Name}
	}

	univers, err := u.univerRepo.GetBenefitByDiplomasUnivers(params, []model.Diploma{*diploma})
	if err != nil {
		return nil, err
	}
	return newUniversShortResponse(univers), nil
}

func (u *UniverService) GetUserDiplomasUnivers(params *dto.UniverBaseParams) ([]dto.UniversityShortResponse, error) {
	uintUserID, ok := params.UserID.(uint)

	if ok && params.FromMyRegion {
		region, err := u.regionRepo.GetUserRegion(uintUserID)
		if err != nil {
			return nil, err
		}
		params.Regions = []string{region.Name}
	}

	diplomas, err := u.diplomaRepo.GetDiplomasByUserID(uintUserID)
	if err != nil {
		return nil, err
	}

	univers, err := u.univerRepo.GetBenefitByDiplomasUnivers(params, diplomas)
	if err != nil {
		return nil, err
	}
	return newUniversShortResponse(univers), nil
}

func (u *UniverService) GetLikedUnivers(userID uint) ([]dto.UniversityShortResponse, error) {
	univers, err := u.univerRepo.GetLikedUnivers(userID)
	if err != nil {
		return nil, err
	}
	return newUniversShortResponse(univers), nil
}

func (u *UniverService) NewUniver(request *dto.UniversityRequest) (uint, error) {
	univerModel := newUniverModel(request)
	return u.univerRepo.NewUniver(univerModel)
}

func (u *UniverService) UpdateUniver(request *dto.UniversityRequest, universityID string) (uint, error) {
	university, err := u.univerRepo.GetUniver(universityID, nil)
	if err != nil {
		return 0, err
	}

	univerModel := newUniverModel(request)
	univerModel.UniversityID = university.UniversityID
	err = u.univerRepo.UpdateUniver(univerModel)

	if err != nil {
		return 0, err
	}
	return university.UniversityID, nil
}

func (u *UniverService) DeleteUniver(universityID string) error {
	univer, err := u.univerRepo.GetUniver(universityID, nil)
	if err != nil {
		return err
	}
	return u.univerRepo.DeleteUniver(univer)
}

func (u *UniverService) LikeUniver(universityID string, userID uint) (bool, error) {
	university, err := u.univerRepo.GetUniver(universityID, userID)
	if err != nil {
		return false, err
	}

	if university.Like {
		return false, nil
	}

	err = u.univerRepo.LikeUniver(university.UniversityID, userID)
	if err != nil {
		return false, err
	}
	u.univerRepo.ChangeUniverPopularity(university, constants.LikeUniverPopularIncr)
	return true, nil
}

func (u *UniverService) DislikeUniver(universityID string, userID uint) (bool, error) {
	university, err := u.univerRepo.GetUniver(universityID, userID)
	if err != nil {
		return false, err
	}

	if !university.Like {
		return false, nil
	}

	err = u.univerRepo.DislikeUniver(university.UniversityID, userID)
	if err != nil {
		return false, err
	}
	u.univerRepo.ChangeUniverPopularity(university, constants.LikeUniverPopularDecr)
	return true, nil
}

func (u *UniverService) UploadLogo(universityID string, file multipart.File, header *multipart.FileHeader) (*dto.UniverLogoResponse, error) {
	university, err := u.univerRepo.GetUniver(universityID, nil)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	if _, err = io.Copy(buf, file); err != nil {
		return nil, err
	}

	ext := filepath.Ext(header.Filename)[1:]
	req := &pb.UploadLogoRequest{
		UniversityId:  universityID,
		FileData:      buf.Bytes(),
		FileExtension: ext,
	}

	resp, err := u.storageServiceClient.UploadLogo(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if resp.Url == "" {
		return nil, errs.StorageServiceError
	}

	university.Logo = resp.Url
	err = u.univerRepo.UpdateUniver(university)
	if err != nil {
		return nil, err
	}
	return &dto.UniverLogoResponse{URL: resp.Url, Name: resp.ObjectName}, nil
}

func (u *UniverService) DeleteLogo(universityID string) error {
	univer, err := u.univerRepo.GetUniver(universityID, nil)
	if err != nil {
		return err
	}

	ext := filepath.Ext(univer.Logo)[1:]
	req := &pb.DeleteLogoRequest{
		UniversityId:  universityID,
		FileExtension: ext,
	}
	_, err = u.storageServiceClient.DeleteLogo(context.Background(), req)
	if err != nil {
		return nil
	}

	univer.Logo = ""
	err = u.univerRepo.UpdateUniver(univer)
	return err
}

func newUniverModel(request *dto.UniversityRequest) *model.University {
	return &model.University{
		Name:        request.Name,
		ShortName:   request.ShortName,
		Logo:        request.Logo,
		Site:        request.Site,
		Email:       request.Email,
		Description: request.Description,
		RegionID:    request.RegionID,
	}
}

func newUniverResponse(univer *model.University) *dto.UniversityResponse {
	return &dto.UniversityResponse{
		Email:       univer.Email,
		Site:        univer.Site,
		Description: univer.Description,
		UniversityShortResponse: dto.UniversityShortResponse{
			UniversityID: univer.UniversityID,
			Name:         univer.Name,
			ShortName:    univer.ShortName,
			Logo:         univer.Logo,
			Region:       univer.Region.Name,
			Like:         univer.Like,
		},
	}
}

func newUniversShortResponse(univers []model.University) []dto.UniversityShortResponse {
	response := make([]dto.UniversityShortResponse, 0, len(univers))
	for _, univer := range univers {
		response = append(response, dto.UniversityShortResponse{
			UniversityID: univer.UniversityID,
			Name:         univer.Name,
			ShortName:    univer.ShortName,
			Logo:         univer.Logo,
			Region:       univer.Region.Name,
			Like:         univer.Like,
		})
	}
	return response
}
