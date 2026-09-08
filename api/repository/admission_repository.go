package repository

import (
	"api/model"
	"encoding/json"

	"gorm.io/gorm"
)

// These are source conditions for review, never automatic diploma eligibility.
// The JSON retains alternatives, exclusions, years, grades and source locators.
func admissionBenefits(db *gorm.DB, query *gorm.DB) ([]model.Benefit, error) {
	var rows []struct {
		ProgramID  uint
		OlympiadID uint
		Relation   string
		Payload    json.RawMessage
	}
	err := query.Select("pr.program_id, ar.olympiad_id, pr.relation, ar.payload").
		Order("ar.olympiad_id, pr.program_id, ar.id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]model.Benefit, 0, len(rows))
	if len(rows) == 0 {
		return result, nil
	}
	pids, oids := map[uint]bool{}, map[uint]bool{}
	for _, r := range rows {
		pids[r.ProgramID] = true
		oids[r.OlympiadID] = true
	}
	pkeys, okeys := make([]uint, 0, len(pids)), make([]uint, 0, len(oids))
	for id := range pids {
		pkeys = append(pkeys, id)
	}
	for id := range oids {
		okeys = append(okeys, id)
	}
	var programs []model.Program
	var olympiads []model.Olympiad
	if err := db.Preload("Field").Preload("University").Where("program_id IN ?", pkeys).Find(&programs).Error; err != nil {
		return nil, err
	}
	if err := db.Where("olympiad_id IN ?", okeys).Find(&olympiads).Error; err != nil {
		return nil, err
	}
	pm, om := map[uint]model.Program{}, map[uint]model.Olympiad{}
	for _, p := range programs {
		pm[p.ProgramID] = p
	}
	for _, o := range olympiads {
		om[o.OlympiadID] = o
	}
	for _, r := range rows {
		var rule struct {
			BenefitTypes []string `json:"benefit_types"`
		}
		if err := json.Unmarshal(r.Payload, &rule); err != nil {
			return nil, err
		}
		bvi := false
		for _, kind := range rule.BenefitTypes {
			if kind == "bvi" {
				bvi = true
			}
		}
		result = append(result, model.Benefit{ProgramID: r.ProgramID, OlympiadID: r.OlympiadID,
			Program: pm[r.ProgramID], Olympiad: om[r.OlympiadID], BVI: bvi,
			AdmissionRule: r.Payload, SourceRelation: r.Relation})
	}
	return result, nil
}

func admissionBenefitQuery(db *gorm.DB) *gorm.DB {
	return db.Table("olympguide.admission_rule AS ar").
		Joins("JOIN olympguide.admission_program_rule pr ON pr.admission_year=ar.admission_year AND pr.rule_id=ar.id").
		Joins("JOIN olympguide.educational_program ep ON ep.program_id=pr.program_id").
		Joins("JOIN olympguide.field_of_study fos ON fos.field_id=ep.field_id").
		Joins("JOIN olympguide.olympiad olymp ON olymp.olympiad_id=ar.olympiad_id").
		Where("ar.admission_year = (SELECT max(admission_year) FROM olympguide.admission_release)")
}
