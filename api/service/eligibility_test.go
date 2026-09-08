package service

import (
	"api/dto"
	"api/model"
	"api/repository"
	"errors"
	"gorm.io/gorm"
	"testing"
)

func intp(i int) *int       { return &i }
func strp(s string) *string { return &s }
func exampleRule() EligibilityRule {
	return EligibilityRule{AdmissionYear: 2026, Benefit: "bvi", Complete: true, Scope: "Точная программа A", ProgramKeys: []string{"A"}, Requirement: Requirement{All: []Requirement{
		{Field: "award_year", Label: "Год диплома", Min: intp(2022), Max: intp(2026)},
		{Field: "profile", Label: "Профиль", Values: []string{"физика"}},
		{Field: "class", Label: "Класс", Values: []string{"11"}},
		{Field: "result", Label: "Результат", Values: []string{"winner", "prize_winner"}},
		{Field: "exam", Label: "ЕГЭ", Subject: "физика", Min: intp(80)},
	}}}
}
func exampleFacts() DiplomaFacts {
	return DiplomaFacts{AwardYear: intp(2026), Profile: strp("физика"), Class: 11, Result: strp("prize_winner")}
}
func exampleProfile() ApplicantProfile {
	return ApplicantProfile{AdmissionYear: intp(2026), Exams: []ExamResult{{Subject: "физика", Score: 80, Year: 2026}}}
}
func TestEligibilityStatesAndReasons(t *testing.T) {
	tests := []struct {
		name, want string
		modify     func(*EligibilityRule, *DiplomaFacts, *ApplicantProfile)
	}{
		{"exact threshold", Eligible, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) {}},
		{"missing confirmation", ConfirmationRequired, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { p.Exams = nil }},
		{"low score", Ineligible, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { p.Exams[0].Score = 79 }},
		{"wrong grade", Ineligible, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { d.Class = 10 }},
		{"unknown award year", ManualReview, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { d.AwardYear = nil }},
		{"expired diploma", Ineligible, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { d.AwardYear = intp(2021) }},
		{"unknown result", ManualReview, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { d.Result = nil }},
		{"participant", Ineligible, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { d.Result = strp("participant") }},
		{"wrong profile", Ineligible, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { d.Profile = strp("химия") }},
		{"never reuse 2026 in 2027", ManualReview, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { p.AdmissionYear = intp(2027) }},
		{"incomplete rule cannot refuse", ManualReview, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { r.Complete = false; d.Class = 1 }},
		{"unmodeled exception", ManualReview, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) {
			r.Requirement.All[4].ManualOnFailure = true
			p.Exams[0].Score = 65
		}},
		{"expired EGE", ConfirmationRequired, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { p.Exams[0].Year = 2021 }},
		{"future EGE", ConfirmationRequired, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { p.Exams[0].Year = 2027 }},
		{"oldest valid EGE", Eligible, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) { p.Exams[0].Year = 2022 }},
		{"malformed rule", ManualReview, func(r *EligibilityRule, d *DiplomaFacts, p *ApplicantProfile) {
			r.Requirement = Requirement{Field: "unrecognized"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, d, p := exampleRule(), exampleFacts(), exampleProfile()
			tt.modify(&r, &d, &p)
			got := EvaluateEligibility(r, d, p, "A", true)
			if got.Status != tt.want || got.Reason == "" {
				t.Fatalf("%+v", got)
			}
			if got.Status == Ineligible {
				found := false
				for _, c := range got.Checks {
					found = found || c.State == "fail" && c.Detail != ""
				}
				if !found {
					t.Fatal("refusal has no failed condition")
				}
			}
		})
	}
	for _, test := range []struct {
		key       string
		unchanged bool
	}{{"other-program-same-faculty", true}, {"A", false}} {
		if got := EvaluateEligibility(exampleRule(), exampleFacts(), exampleProfile(), test.key, test.unchanged); got.Status != ManualReview {
			t.Fatal(got)
		}
	}
}
func TestAlternativesRemainJointAndTraceSelectedBranch(t *testing.T) {
	r := exampleRule()
	r.Requirement = Requirement{Any: []Requirement{
		{All: []Requirement{{Field: "result", Label: "Победитель", Values: []string{"winner"}}, {Field: "exam", Label: "Физика 75", Subject: "физика", Min: intp(75)}}},
		{All: []Requirement{{Field: "result", Label: "Призер", Values: []string{"prize_winner"}}, {Field: "exam", Label: "Физика 85", Subject: "физика", Min: intp(85)}}},
	}}
	p := exampleProfile()
	d := exampleFacts()
	if got := EvaluateEligibility(r, d, p, "A", true); got.Status != Ineligible {
		t.Fatalf("cross-joined winner's threshold: %+v", got)
	}
	p.Exams[0].Score = 85
	got := EvaluateEligibility(r, d, p, "A", true)
	if got.Status != Eligible {
		t.Fatal(got)
	}
	for _, c := range got.Checks {
		if c.State == "fail" {
			t.Fatal("trace presents unused failed branch as a condition", got)
		}
	}
}
func TestDiplomaAndExamValidation(t *testing.T) {
	if err := ValidateDiplomaDetails(dto.DiplomaDetails{}, 9, 2); err != nil {
		t.Fatal("legacy request rejected", err)
	}
	if err := ValidateDiplomaDetails(dto.DiplomaDetails{AwardYear: intp(2026), OlympiadYear: strp("2024/2025")}, 11, 1); err == nil {
		t.Fatal("inconsistent season accepted")
	}
	if err := ValidateDiplomaDetails(dto.DiplomaDetails{Result: strp("winner")}, 11, 2); err == nil {
		t.Fatal("inconsistent degree accepted")
	}
	p := exampleProfile()
	p.Exams = append(p.Exams, p.Exams[0])
	if ValidateApplicantProfile(p) == nil {
		t.Fatal("duplicate exam accepted")
	}
	p = exampleProfile()
	p.Exams[0].Score = 101
	if ValidateApplicantProfile(p) == nil {
		t.Fatal("bad score accepted")
	}
}

type ownedDiplomaRepo struct{ repository.IDiplomaRepo }

func (ownedDiplomaRepo) GetDiplomaByID(string) (*model.Diploma, error) {
	return &model.Diploma{UserID: 42}, nil
}
func TestLegacyDiplomaQueriesEnforceOwnership(t *testing.T) {
	repo := ownedDiplomaRepo{}
	// Nil downstream repos deliberately panic if authorization fails open.
	b := NewBenefitService(nil, repo)
	u := NewUniverService(nil, nil, repo, nil)
	for _, id := range []any{nil, uint(0), uint(7), "42"} {
		if _, err := b.GetBenefitsByDiploma("1", &dto.BenefitByOlympiadQueryParams{UserID: id}); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatal(err)
		}
		if _, err := u.GetDiplomaUnivers(&dto.UniverBaseParams{UserID: id}, "1"); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatal(err)
		}
	}
}
