package handler

import (
	"api/dto"
	"api/repository"
	"api/service"
	"encoding/json"
	"fmt"
	"testing"
)

func TestFieldDirectoryOnlyIncludesFieldsWithPrograms(t *testing.T) {
	db := personalTestDB(t)
	// A linked and an unlinked field in the same group: no dependence on the imported catalogue.
	if err := db.Exec(`INSERT INTO olympguide.group_of_fields(name,code) VALUES('Catalogue test','99.00.00') ON CONFLICT DO NOTHING;
      INSERT INTO olympguide.field_of_study(name,code,degree,group_id) SELECT 'Catalogue test empty','99.03.91','Бакалавриат',group_id FROM olympguide.group_of_fields WHERE code='99.00.00';
      INSERT INTO olympguide.field_of_study(name,code,degree,group_id) SELECT 'Catalogue test linked','99.03.92','Бакалавриат',group_id FROM olympguide.group_of_fields WHERE code='99.00.00';`).Error; err != nil {
		t.Fatal(err)
	}
	var uid uint
	if err := db.Raw(`INSERT INTO olympguide.university(name,region_id) SELECT 'Catalogue test',min(region_id) FROM olympguide.region RETURNING university_id`).Scan(&uid).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO olympguide.educational_program(name,university_id,field_id) SELECT 'Catalogue test program',?,field_id FROM olympguide.field_of_study WHERE code='99.03.92'`, uid).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.NewFieldService(repository.NewPgFieldRepo(db))
	rows, err := svc.GetGroups(&dto.GroupQueryParams{Search: "Catalogue test", Degrees: []string{"Бакалавриат"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0].Fields) != 1 || rows[0].Fields[0].Code != "99.03.92" {
		t.Fatalf("Unlinked field leaked: %+v", rows)
	}
	rows, err = svc.GetGroups(&dto.GroupQueryParams{Search: "Catalogue test empty"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(rows)
	if string(data) != "[]" {
		t.Fatalf("Empty list must be an array: %s", data)
	}
}

func TestOlympiadUniversitiesRequireProgramLinks(t *testing.T) {
	db := personalTestDB(t)
	var oid, uid, pid uint
	if err := db.Raw(`SELECT olympiad_id FROM olympguide.admission_rule ORDER BY admission_year DESC LIMIT 1`).Scan(&oid).Error; err != nil {
		t.Fatal(err)
	}
	if oid == 0 {
		t.Skip("Import the admissions fixture first")
	}
	// Remove all links in a rollback-only transaction, retaining source rules.
	if err := db.Exec(`DELETE FROM olympguide.benefit WHERE olympiad_id=?`, oid).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DELETE FROM olympguide.admission_program_rule pr USING olympguide.admission_rule ar WHERE ar.id=pr.rule_id AND ar.admission_year=pr.admission_year AND ar.olympiad_id=?`, oid).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewPgUniverRepo(db)
	rows, err := repo.GetBenefitByOlympUnivers(&dto.UniverBaseParams{}, fmt.Sprint(oid))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("Unscoped rules must not create empty university headings: %d", len(rows))
	}
	if err := db.Raw(`SELECT university_id FROM olympguide.admission_rule WHERE olympiad_id=? ORDER BY admission_year DESC LIMIT 1`, oid).Scan(&uid).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT program_id FROM olympguide.educational_program WHERE university_id=? LIMIT 1`, uid).Scan(&pid).Error; err != nil {
		t.Fatal(err)
	}
	if pid == 0 {
		t.Fatal("fixture requires a program")
	}
	if err := db.Exec(`INSERT INTO olympguide.admission_program_rule(admission_year,rule_id,program_id,relation) SELECT admission_year,id,?,'program_conditions' FROM olympguide.admission_rule WHERE olympiad_id=? AND university_id=? AND admission_year=(SELECT max(admission_year) FROM olympguide.admission_release) LIMIT 1`, pid, oid, uid).Error; err != nil {
		t.Fatal(err)
	}
	rows, err = repo.GetBenefitByOlympUnivers(&dto.UniverBaseParams{}, fmt.Sprint(oid))
	if err != nil || len(rows) != 1 || rows[0].UniversityID != uid {
		t.Fatalf("Expected linked university: %+v, %v", rows, err)
	}
	benefits, err := repository.NewPgBenefitRepo(db).GetBenefitsByOlympiad(fmt.Sprint(oid), &dto.BenefitByOlympiadQueryParams{UniversityID: uid})
	if err != nil || len(benefits) == 0 {
		t.Fatalf("Listed university must expand: %v", err)
	}
	for _, b := range benefits {
		if b.Program.UniversityID != uid {
			t.Fatal("Foreign university program leaked")
		}
	}
}

func TestRegistryCatalogSeparatesSeasonsAndRetainsHistoricalIDs(t *testing.T) {
	db := personalTestDB(t)
	repo := repository.NewPgOlympRepo(db)
	rows, err := repo.GetOlymps(&dto.OlympQueryParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Skip("Import the reviewed RSOSH registry fixture")
	}
	if len(rows) != 303 {
		t.Fatalf("Want 303 profiles, got %d", len(rows))
	}
	for _, o := range rows {
		if o.Category != "rsosh" || o.AcademicYear != "2026/2027" || o.RegistryStatus != "draft" || o.AdmissionYear != nil || o.Level < 1 || o.Level > 3 || o.Profile == "" || o.Subjects == "" {
			t.Fatalf("Wrong season or incomplete profile: %+v", o)
		}
	}
	filtered, err := repo.GetOlymps(&dto.OlympQueryParams{Levels: []uint{3}, Profiles: []string{"беспилотные авиационные системы"}})
	if err != nil || len(filtered) != 1 {
		t.Fatalf("New NTO profile/level filtering failed: %d, %v", len(filtered), err)
	}
	var historical uint
	if err := db.Raw("SELECT olympiad_id FROM olympguide.admission_rule LIMIT 1").Scan(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if historical == 0 {
		t.Fatal("Historical fixture missing")
	}
	old, err := repo.GetOlymp(fmt.Sprint(historical), nil)
	if err != nil || old.AcademicYear != "" {
		t.Fatalf("Historical diploma identity lost: %v", err)
	}
	previousPopularity := old.Popularity
	repo.ChangeOlympPopularity(old, 1)
	refreshed, err := repo.GetOlymp(fmt.Sprint(historical), nil)
	if err != nil || refreshed.Popularity != previousPopularity+1 || refreshed.AcademicYear != "" {
		t.Fatalf("Updating an old favourite must preserve nullable registry metadata: %v", err)
	}
	profiles, err := repo.GetOlympiadProfiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		found := false
		for _, o := range rows {
			if o.Profile == profile {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Old profile in current filter: %s", profile)
		}
	}
	svc := service.NewOlympService(repo)
	empty, err := svc.GetOlymps(&dto.OlympQueryParams{Search: "no-such-registry-olympiad"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(empty)
	if string(data) != "[]" {
		t.Fatalf("Empty list must be an array: %s", data)
	}
}
