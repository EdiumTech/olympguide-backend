package handler

import (
	"api/dto"
	"api/model"
	"api/repository"
	"api/service"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type universityListRepo struct {
	repository.IUniverRepo
	universities []model.University
	err          error
}

func (r universityListRepo) GetUnivers(*dto.UniverBaseParams) ([]model.University, error) {
	return r.universities, r.err
}

func TestGetUniversResponseContract(t *testing.T) {
	for _, test := range []struct {
		name         string
		universities []model.University
		err          error
		status       int
	}{
		{name: "nil result", status: http.StatusOK},
		{name: "empty search result", universities: []model.University{}, status: http.StatusOK},
		{name: "populated result", universities: []model.University{{UniversityID: 7, Name: "University", ShortName: "U", Like: true}}, status: http.StatusOK},
		{name: "database error", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := universityListRepo{universities: test.universities, err: test.err}
			handler := NewUniverHandler(service.NewUniverService(repo, nil, nil, nil))
			router := gin.New()
			router.GET("/api/v1/universities", handler.GetUnivers)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/universities?search=test", nil))
			if response.Code != test.status {
				t.Fatalf("status %d, expected %d: %s", response.Code, test.status, response.Body.String())
			}
			if test.err != nil {
				return
			}
			if len(test.universities) == 0 {
				if response.Body.String() != "[]" {
					t.Fatalf("empty university list must be [], got %s", response.Body.String())
				}
				return
			}
			var universities []dto.UniversityShortResponse
			if err := json.Unmarshal(response.Body.Bytes(), &universities); err != nil {
				t.Fatal(err)
			}
			if len(universities) != 1 || universities[0].UniversityID != 7 || universities[0].Name != "University" || !universities[0].Like {
				t.Fatalf("populated response changed: %s", response.Body.String())
			}
		})
	}
}
