package service

import "testing"

func TestFilterParsedWorldBookForUserHidesSchedulerOnlyEntries(t *testing.T) {
	book := ParsedWorldBook{
		GlobalEnabled: true,
		MainEntries: []ParsedWorldBookEntry{
			{ID: "public", Enabled: true, UserVisible: true, SchedulerEnabled: false},
			{ID: "hidden", Enabled: true, UserVisible: false, SchedulerEnabled: true},
			{ID: "disabled", Enabled: false, UserVisible: true, SchedulerEnabled: false},
		},
		SubEntries: []ParsedWorldBookEntry{
			{ID: "public-sub", Enabled: true, UserVisible: true, SchedulerEnabled: true},
		},
	}

	filtered := FilterParsedWorldBookForUser(book)
	if len(filtered.MainEntries) != 1 || filtered.MainEntries[0].ID != "public" {
		t.Fatalf("unexpected user main entries: %+v", filtered.MainEntries)
	}
	if len(filtered.SubEntries) != 1 || filtered.SubEntries[0].ID != "public-sub" {
		t.Fatalf("unexpected user sub entries: %+v", filtered.SubEntries)
	}
}

func TestFilterParsedWorldBookForSchedulerKeepsOnlyEnabledSchedulerEntries(t *testing.T) {
	book := ParsedWorldBook{
		GlobalEnabled: true,
		MainEntries: []ParsedWorldBookEntry{
			{ID: "public-rule", Enabled: true, UserVisible: true, SchedulerEnabled: false},
			{ID: "scheduler-main", Enabled: true, UserVisible: false, SchedulerEnabled: true},
		},
		SubEntries: []ParsedWorldBookEntry{
			{ID: "disabled-scheduler", Enabled: false, UserVisible: false, SchedulerEnabled: true},
			{ID: "scheduler-sub", Enabled: true, UserVisible: false, SchedulerEnabled: true},
		},
	}

	filtered := FilterParsedWorldBookForScheduler(book)
	if len(filtered.MainEntries) != 1 || filtered.MainEntries[0].ID != "scheduler-main" {
		t.Fatalf("unexpected scheduler main entries: %+v", filtered.MainEntries)
	}
	if len(filtered.SubEntries) != 1 || filtered.SubEntries[0].ID != "scheduler-sub" {
		t.Fatalf("unexpected scheduler sub entries: %+v", filtered.SubEntries)
	}
}

func TestFilterParsedWorldBookDisablesEverythingWhenBookIsGlobalDisabled(t *testing.T) {
	book := ParsedWorldBook{
		GlobalEnabled: false,
		MainEntries:   []ParsedWorldBookEntry{{ID: "public", Enabled: true, UserVisible: true, SchedulerEnabled: true}},
		SubEntries:    []ParsedWorldBookEntry{{ID: "hidden", Enabled: true, UserVisible: false, SchedulerEnabled: true}},
	}
	if got := FilterParsedWorldBookForUser(book); len(got.MainEntries)+len(got.SubEntries) != 0 {
		t.Fatalf("global-disabled book leaked to user: %+v", got)
	}
	if got := FilterParsedWorldBookForScheduler(book); len(got.MainEntries)+len(got.SubEntries) != 0 {
		t.Fatalf("global-disabled book leaked to scheduler: %+v", got)
	}
}
