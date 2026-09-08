package service

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"
)

type CalendarEvent struct {
	ID             string   `json:"id,omitempty"`
	Title          string   `json:"title"`
	Kind           string   `json:"kind"`
	Season         string   `json:"season"`
	AdmissionYear  *int     `json:"admission_year"`
	TimeKind       string   `json:"time_kind"`
	Timezone       string   `json:"timezone"`
	StartDate      string   `json:"start_date,omitempty"`
	EndDate        string   `json:"end_date,omitempty"`
	StartsAt       string   `json:"starts_at,omitempty"`
	EndsAt         string   `json:"ends_at,omitempty"`
	SourceURL      string   `json:"source_url,omitempty"`
	CheckedOn      string   `json:"checked_on,omitempty"`
	Description    string   `json:"description"`
	UniversityKey  string   `json:"university_key,omitempty"`
	ProgramKeys    []string `json:"program_keys,omitempty"`
	OlympiadKeys   []string `json:"olympiad_keys,omitempty"`
	WebOlympiadIDs []string `json:"web_olympiad_ids,omitempty"`
}

func CalendarStatus(s string) bool {
	return s == "planned" || s == "registered" || s == "participating" || s == "completed" || s == "skipped"
}
func CalendarKind(s string) bool {
	switch s {
	case "registration_open", "registration_close", "qualifying", "final", "results", "appeal", "documents_open", "documents_close", "admission", "custom":
		return true
	}
	return false
}

// CalendarBounds returns a half-open instant interval. Date ranges include both
// named dates in the event's zone. Undated events have no invented placeholder.
func CalendarBounds(e CalendarEvent) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(e.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("Неизвестный часовой пояс")
	}
	fail := func() (time.Time, time.Time, error) {
		return time.Time{}, time.Time{}, fmt.Errorf("Некорректные даты события")
	}
	switch e.TimeKind {
	case "undated":
		if e.StartDate != "" || e.EndDate != "" || e.StartsAt != "" || e.EndsAt != "" {
			return fail()
		}
		return time.Time{}, time.Time{}, nil
	case "all_day", "date_range":
		if e.StartsAt != "" || e.EndsAt != "" {
			return fail()
		}
		start, err := time.ParseInLocation(time.DateOnly, e.StartDate, loc)
		if err != nil {
			return fail()
		}
		end := start
		if e.TimeKind == "date_range" {
			end, err = time.ParseInLocation(time.DateOnly, e.EndDate, loc)
			if err != nil || end.Before(start) {
				return fail()
			}
		} else if e.EndDate != "" {
			return fail()
		}
		return start, end.AddDate(0, 0, 1), nil
	case "timed":
		if e.StartDate != "" || e.EndDate != "" {
			return fail()
		}
		parse := func(v string) (time.Time, error) {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return t, err
			}
			_, given := t.Zone()
			_, actual := t.In(loc).Zone()
			if given != actual {
				return t, fmt.Errorf("Смещение времени не соответствует часовому поясу")
			}
			return t, nil
		}
		start, err := parse(e.StartsAt)
		if err != nil {
			return fail()
		}
		end := start.Add(time.Nanosecond)
		if e.EndsAt != "" {
			end, err = parse(e.EndsAt)
			if err != nil || !end.After(start) {
				return fail()
			}
		}
		return start, end, nil
	}
	return fail()
}

func ValidateCalendarEvent(e CalendarEvent, custom bool) error {
	if strings.TrimSpace(e.Title) == "" || len([]rune(e.Title)) > 240 || len([]rune(e.Description)) > 8000 || !CalendarKind(e.Kind) {
		return fmt.Errorf("Укажите название и вид события")
	}
	if e.Season != "" {
		if len(e.Season) != 9 {
			return fmt.Errorf("Учебный год: ГГГГ/ГГГГ")
		}
		a, err := time.Parse("2006", e.Season[:4])
		b, err2 := time.Parse("2006", e.Season[5:])
		if err != nil || err2 != nil || e.Season[4] != '/' || b.Year() != a.Year()+1 {
			return fmt.Errorf("Некорректный учебный год")
		}
	}
	if e.AdmissionYear != nil && (*e.AdmissionYear < 2000 || *e.AdmissionYear > 2100) {
		return fmt.Errorf("Некорректный год поступления")
	}
	if !custom && (e.SourceURL == "" || e.CheckedOn == "") {
		return fmt.Errorf("Нет источника события")
	}
	if e.SourceURL != "" {
		u, err := url.Parse(e.SourceURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return fmt.Errorf("Нужна HTTPS-ссылка")
		}
	}
	_, _, err := CalendarBounds(e)
	return err
}
