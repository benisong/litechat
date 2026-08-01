package service

import "testing"

func TestEvaluateConditionGroupAllComparisons(t *testing.T) {
	state := map[string]any{
		"world.jiang_muchen.injured":    true,
		"relationships.liu_ruyan.trust": 85,
	}
	group := ConditionGroup{All: []Condition{
		{Field: "world.jiang_muchen.injured", Operator: "eq", Value: true},
		{Field: "relationships.liu_ruyan.trust", Operator: "gte", Value: 80},
	}}
	matched, err := EvaluateConditionGroup(group, state)
	if err != nil {
		t.Fatalf("EvaluateConditionGroup: %v", err)
	}
	if !matched {
		t.Fatal("expected all conditions to match")
	}
}

func TestEvaluateConditionGroupAnyAndNone(t *testing.T) {
	state := map[string]any{"route": "survival", "facts.resource_request": true}
	group := ConditionGroup{
		Any: []Condition{
			{Field: "route", Operator: "eq", Value: "blood_path"},
			{Field: "facts.resource_request", Operator: "eq", Value: true},
		},
		None: []Condition{
			{Field: "events.resource_dispute_001", Operator: "exists"},
		},
	}
	matched, err := EvaluateConditionGroup(group, state)
	if err != nil {
		t.Fatalf("EvaluateConditionGroup: %v", err)
	}
	if !matched {
		t.Fatal("expected any/none conditions to match")
	}
}

func TestEvaluateConditionGroupRejectsInvalidOperator(t *testing.T) {
	_, err := EvaluateConditionGroup(ConditionGroup{
		All: []Condition{{Field: "route", Operator: "contains_regex", Value: "x"}},
	}, map[string]any{"route": "survival"})
	if err == nil {
		t.Fatal("expected invalid operator to fail")
	}
}

func TestEvaluateConditionGroupMissingFieldBehavior(t *testing.T) {
	state := map[string]any{}
	matched, err := EvaluateConditionGroup(ConditionGroup{
		All: []Condition{{Field: "missing", Operator: "not_exists"}},
	}, state)
	if err != nil {
		t.Fatalf("EvaluateConditionGroup: %v", err)
	}
	if !matched {
		t.Fatal("expected not_exists to match")
	}
}
