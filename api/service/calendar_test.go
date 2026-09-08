package service

import (
	"testing"
	"time"
)

func TestCalendarPrecisionAndZones(t *testing.T) {
	valid := []CalendarEvent{
		{TimeKind: "undated", Timezone: "Europe/Moscow"},
		{TimeKind: "all_day", Timezone: "Europe/Berlin", StartDate: "2026-03-29"},
		{TimeKind: "date_range", Timezone: "Europe/Moscow", StartDate: "2026-12-31", EndDate: "2027-01-02"},
		{TimeKind: "timed", Timezone: "Europe/Moscow", StartsAt: "2026-09-21T14:00:00+03:00"},
	}
	for _, e := range valid {
		if _, _, err := CalendarBounds(e); err != nil {
			t.Fatal(err)
		}
	}
	start, end, _ := CalendarBounds(valid[1])
	if end.Sub(start) != 23*time.Hour {
		t.Fatal("all day must follow DST, not add 24h")
	}
	start, end, _ = CalendarBounds(valid[2])
	if end.Sub(start) != 72*time.Hour {
		t.Fatal("inclusive date range")
	}
	invalid := []CalendarEvent{
		{TimeKind: "undated", Timezone: "Europe/Moscow", StartDate: "2026-01-01"},
		{TimeKind: "all_day", Timezone: "Europe/Moscow", StartDate: "2026-02-29"},
		{TimeKind: "date_range", Timezone: "Europe/Moscow", StartDate: "2026-02-02", EndDate: "2026-02-01"},
		{TimeKind: "timed", Timezone: "Europe/Moscow", StartsAt: "2026-09-21T14:00:00Z"},
		{TimeKind: "timed", Timezone: "Europe/Moscow", StartsAt: "2026-09-21T14:00:00"},
		{TimeKind: "all_day", Timezone: "Imaginary/Zone", StartDate: "2026-01-01"},
	}
	for _, e := range invalid {
		if _, _, err := CalendarBounds(e); err == nil {
			t.Fatalf("accepted %+v", e)
		}
	}
}
