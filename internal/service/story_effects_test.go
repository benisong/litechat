package service

import "testing"

func TestApplyStateEffectsUpdatesWritableBoundedInteger(t *testing.T) {
	state := map[string]any{"trust": 80}
	spec := map[string]FieldSpec{
		"trust": {Type: "integer", Writable: true, Min: 0, Max: 100, HasMin: true, HasMax: true},
	}
	err := ApplyStateEffects(state, []StateEffect{{Field: "trust", Operation: "increment", Value: 30}}, spec)
	if err != nil {
		t.Fatalf("ApplyStateEffects: %v", err)
	}
	if state["trust"] != 100 {
		t.Fatalf("expected clamped trust 100, got %v", state["trust"])
	}
}

func TestApplyStateEffectsSupportsEnumSetAndAppend(t *testing.T) {
	state := map[string]any{"route": "survival", "clues": []any{"first"}}
	spec := map[string]FieldSpec{
		"route": {Type: "enum", Writable: true, Allowed: map[string]bool{"survival": true, "blood_path": true}},
		"clues": {Type: "string_set", Writable: true},
	}
	err := ApplyStateEffects(state, []StateEffect{
		{Field: "route", Operation: "set", Value: "blood_path"},
		{Field: "clues", Operation: "append", Value: "second"},
		{Field: "clues", Operation: "append", Value: "second"},
	}, spec)
	if err != nil {
		t.Fatalf("ApplyStateEffects: %v", err)
	}
	if state["route"] != "blood_path" {
		t.Fatalf("unexpected route: %v", state["route"])
	}
	clues := state["clues"].([]any)
	if len(clues) != 2 {
		t.Fatalf("expected deduplicated clues, got %#v", clues)
	}
}

func TestApplyStateEffectsRejectsUndeclaredOrReadOnlyField(t *testing.T) {
	state := map[string]any{"secret": false}
	spec := map[string]FieldSpec{"secret": {Type: "boolean", Writable: false}}
	if err := ApplyStateEffects(state, []StateEffect{{Field: "secret", Operation: "set", Value: true}}, spec); err == nil {
		t.Fatal("expected read-only field to fail")
	}
	if err := ApplyStateEffects(state, []StateEffect{{Field: "unknown", Operation: "set", Value: true}}, spec); err == nil {
		t.Fatal("expected undeclared field to fail")
	}
}
