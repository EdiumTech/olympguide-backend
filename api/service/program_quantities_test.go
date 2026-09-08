package service

import (
	"api/dto"
	"api/model"
	"encoding/json"
	"testing"
)

func TestProgramQuantitiesDistinguishUnknownFromZero(t *testing.T) {
	zero, places, cost := uint(0), uint(25), uint(350000)
	cases := []struct {
		name    string
		program model.Program
		want    []any
	}{
		{"unknown", model.Program{}, []any{nil, nil, nil}},
		{"confirmed zero", model.Program{BudgetPlaces: &zero, PaidPlaces: &zero, Cost: &zero}, []any{float64(0), float64(0), float64(0)}},
		{"partially known", model.Program{BudgetPlaces: &places, Cost: &cost}, []any{float64(25), nil, float64(350000)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The same serializer serves list entries and the detail response.
			for _, response := range []any{newProgramShortResponse(&tc.program), newProgramResponse(&tc.program)} {
				encoded, err := json.Marshal(response)
				if err != nil {
					t.Fatal(err)
				}
				var object map[string]any
				if err := json.Unmarshal(encoded, &object); err != nil {
					t.Fatal(err)
				}
				for i, key := range []string{"budget_places", "paid_places", "cost"} {
					got, exists := object[key]
					if !exists || got != tc.want[i] {
						t.Fatalf("%s: got %v, want %v", key, got, tc.want[i])
					}
				}
			}
		})
	}
}

func TestNewProgramRetainsMissingAndExplicitZeroQuantities(t *testing.T) {
	for _, payload := range []string{`{"name":"P"}`, `{"name":"P","budget_places":null}`, `{"name":"P","budget_places":0}`} {
		var request dto.ProgramRequest
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			t.Fatal(err)
		}
		program := newProgramModel(&request)
		if (program.BudgetPlaces == nil) != (request.BudgetPlaces == nil) {
			t.Fatal("lost missing quantity")
		}
		if request.BudgetPlaces != nil && *program.BudgetPlaces != 0 {
			t.Fatal("lost confirmed zero")
		}
	}
}
