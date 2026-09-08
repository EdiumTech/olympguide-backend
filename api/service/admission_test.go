package service

import (
	"api/model"
	"encoding/json"
	"testing"
)

func TestAdmissionConditionsAreNotFlattened(t *testing.T) {
	raw := json.RawMessage(`{"id":"rule-1","benefit_types":["bvi","100_points"],"values":{"grades":"11; exceptions for grade 10","program_scope":"All except 01.03.01"},"conditions":["Alternatives apply together"],"source_url":"https://example.org/official.pdf"}`)
	info := extractBenefitInfo(model.Benefit{AdmissionRule: raw, SourceRelation: "school_conditions", BVI: true})
	if info.MinClass != nil || info.MinDiplomaLevel != nil {
		t.Fatal("source rule was converted into an unconditional minimum")
	}
	if string(info.AdmissionRule) != string(raw) || info.SourceRelation != "school_conditions" {
		t.Fatal("source conditions were lost")
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["admission_rule"].(map[string]any); !ok {
		t.Fatal("rule is not a JSON object")
	}
	legacy := extractBenefitInfo(model.Benefit{MinClass: 10, MinDiplomaLevel: 3})
	if *legacy.MinClass != 10 || *legacy.MinDiplomaLevel != 3 {
		t.Fatal("legacy benefits changed")
	}
}

func TestAdmissionTreesGroupNonAdjacentRows(t *testing.T) {
	rows := []model.Benefit{{ProgramID: 1, OlympiadID: 2}, {ProgramID: 3, OlympiadID: 4}, {ProgramID: 1, OlympiadID: 2}}
	programs := newProgramBenefitTrees(rows)
	olympiads := newOlympiadBenefitTrees(rows)
	if len(programs) != 2 || len(programs[0].Benefits) != 2 || len(olympiads) != 2 || len(olympiads[0].Benefits) != 2 {
		t.Fatal("duplicate entity groups")
	}
}

func TestSharedProgramAppearsInBothFaculties(t *testing.T) {
	parent := uint(1)
	program := model.Program{ProgramID: 5, Faculties: []model.Faculty{{FacultyID: 1, Name: "A"}, {FacultyID: 2, Name: "B"}, {FacultyID: 3, Name: "Department", ParentID: &parent}}}
	trees := newFacultyProgramTree([]model.Program{program, {ProgramID: 6}})
	if len(trees) != 3 || trees[0].Programs[0].ProgramID != 5 || trees[1].Programs[0].ProgramID != 5 || trees[2].Programs[0].ProgramID != 6 {
		t.Fatal("lost a shared or unaffiliated program")
	}
}
