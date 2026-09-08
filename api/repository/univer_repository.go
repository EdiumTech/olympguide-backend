package repository

import (
	"api/dto"
	"api/model"
	"gorm.io/gorm"
	"strings"
)

type IUniverRepo interface {
	UniverExists(univerID uint) bool
	GetUniver(universityID string, userID any) (*model.University, error)
	GetUnivers(params *dto.UniverBaseParams) ([]model.University, error)
	GetBenefitByOlympUnivers(params *dto.UniverBaseParams, olympiadID string) ([]model.University, error)
	GetBenefitByDiplomasUnivers(params *dto.UniverBaseParams, diplomas []model.Diploma) ([]model.University, error)
	GetLikedUnivers(userID uint) ([]model.University, error)
	NewUniver(univer *model.University) (uint, error)
	UpdateUniver(univer *model.University) error
	DeleteUniver(univer *model.University) error
	ChangeUniverPopularity(university *model.University, value int)
	LikeUniver(universityID uint, userID uint) error
	DislikeUniver(universityID uint, userID uint) error
	Exists(universityID uint) bool
}

type PgUniverRepo struct {
	db *gorm.DB
}

func NewPgUniverRepo(db *gorm.DB) *PgUniverRepo {
	return &PgUniverRepo{db: db}
}

func (u *PgUniverRepo) UniverExists(univerID uint) bool {
	var univerExists bool
	u.db.Raw("SELECT EXISTS(SELECT 1 FROM olympguide.university WHERE university_id = ?)", univerID).Scan(&univerExists)
	return univerExists
}

func (u *PgUniverRepo) GetUniver(universityID string, userID any) (*model.University, error) {
	var university model.University
	err := u.db.Debug().Preload("Region").
		Joins("LEFT JOIN olympguide.liked_universities lu ON lu.university_id = olympguide.university.university_id AND lu.user_id = ?", userID).
		Select("olympguide.university.*, CASE WHEN lu.user_id IS NOT NULL THEN TRUE ELSE FALSE END as like").
		Where("olympguide.university.university_id = ?", universityID).
		First(&university).Error
	if err != nil {
		return nil, err
	}
	return &university, nil
}

func (u *PgUniverRepo) GetUnivers(params *dto.UniverBaseParams) ([]model.University, error) {
	var universities []model.University
	query := u.buildUniversQuery(params)
	if err := query.Order("popularity DESC").Find(&universities).Error; err != nil {
		return nil, err
	}
	return universities, nil
}

func (u *PgUniverRepo) GetBenefitByOlympUnivers(params *dto.UniverBaseParams, olympiadID string) ([]model.University, error) {
	var universities []model.University
	query := u.buildUniversQuery(params).Where("olympguide.university.university_id IN ("+
		"SELECT DISTINCT u.university_id "+
		"FROM olympguide.benefit AS b "+
		"JOIN olympguide.educational_program AS pr ON pr.program_id = b.program_id "+
		"JOIN olympguide.university AS u ON u.university_id = pr.university_id "+
		"WHERE b.olympiad_id = ? UNION SELECT university_id FROM olympguide.admission_rule "+
		"WHERE olympiad_id = ? AND admission_year = (SELECT max(admission_year) FROM olympguide.admission_release))", olympiadID, olympiadID)

	if err := query.Order("popularity DESC").Find(&universities).Error; err != nil {
		return nil, err
	}
	return universities, nil
}

func (u *PgUniverRepo) GetBenefitByDiplomasUnivers(params *dto.UniverBaseParams, diplomas []model.Diploma) ([]model.University, error) {
	var universities []model.University

	if len(diplomas) == 0 {
		return []model.University{}, nil
	}

	var conditions []string
	var queryParams []interface{}
	for _, diploma := range diplomas {
		conditions = append(conditions, "(b.min_class <= ? AND b.min_diploma_level >= ? AND b.olympiad_id = ?)")
		queryParams = append(queryParams, diploma.Class, diploma.Level, diploma.OlympiadID)
	}

	whereClause := strings.Join(conditions, " OR ")
	subQuery := "SELECT DISTINCT u.university_id " +
		"FROM olympguide.benefit AS b " +
		"JOIN olympguide.educational_program AS pr ON pr.program_id = b.program_id " +
		"JOIN olympguide.university AS u ON u.university_id = pr.university_id " +
		"WHERE " + whereClause

	query := u.buildUniversQuery(params).
		Where("olympguide.university.university_id IN ("+subQuery+")", queryParams...)

	if err := query.Order("popularity DESC").Find(&universities).Error; err != nil {
		return nil, err
	}
	return universities, nil
}

func (u *PgUniverRepo) GetLikedUnivers(userID uint) ([]model.University, error) {
	var universities []model.University
	err := u.db.Debug().Preload("Region").
		Joins("LEFT JOIN olympguide.liked_universities lu ON lu.university_id = olympguide.university.university_id AND lu.user_id = ?", userID).
		Where("lu.user_id IS NOT NULL").
		Select("olympguide.university.*, TRUE as like").
		Order("popularity DESC").
		Find(&universities).Error

	if err != nil {
		return nil, err
	}
	return universities, nil
}

func (u *PgUniverRepo) NewUniver(univer *model.University) (uint, error) {
	err := u.db.Create(&univer).Error
	if err != nil {
		return 0, err
	}
	return univer.UniversityID, nil
}

func (u *PgUniverRepo) UpdateUniver(univer *model.University) error {
	return u.db.Save(univer).Error
}

func (u *PgUniverRepo) DeleteUniver(univer *model.University) error {
	return u.db.Delete(univer).Error
}

func (u *PgUniverRepo) ChangeUniverPopularity(university *model.University, value int) {
	university.Popularity += value
	u.db.Save(university)
}

func (u *PgUniverRepo) LikeUniver(universityID uint, userID uint) error {
	likedUniversity := model.LikedUniversities{
		UniversityID: universityID,
		UserID:       userID,
	}
	err := u.db.Create(&likedUniversity).Error
	return err
}

func (u *PgUniverRepo) DislikeUniver(universityID uint, userID uint) error {
	likedUniversity := model.LikedUniversities{
		UniversityID: universityID,
		UserID:       userID,
	}
	err := u.db.Delete(&likedUniversity).Error
	return err
}

func (u *PgUniverRepo) Exists(universityID uint) bool {
	var count int64
	u.db.Model(&model.University{}).Where("university_id = ?", universityID).Count(&count)
	return count > 0
}

func (u *PgUniverRepo) buildUniversQuery(params *dto.UniverBaseParams) *gorm.DB {
	query := u.db.Debug().Preload("Region").
		Joins("LEFT JOIN olympguide.liked_universities lu "+
			"ON lu.university_id = olympguide.university.university_id AND lu.user_id = ?", params.UserID).
		Joins("LEFT JOIN olympguide.region r ON r.region_id = olympguide.university.region_id").
		Select("olympguide.university.*, CASE WHEN lu.user_id IS NOT NULL THEN TRUE ELSE FALSE END as like")

	if params.Search != "" {
		query = query.Where("olympguide.university.name ILIKE ? OR short_name ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	if len(params.Regions) > 0 {
		query = query.Where("r.name IN (?)", params.Regions)
	}

	return query
}
