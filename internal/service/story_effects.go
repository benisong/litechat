package service

import (
	"fmt"
	"strings"
)

// FieldSpec 是经过 Manifest 校验后的动态字段白名单。
type FieldSpec struct {
	Type     string
	Writable bool
	Min      float64
	Max      float64
	HasMin   bool
	HasMax   bool
	Allowed  map[string]bool
}

// StateEffect 是规则引擎执行的确定性状态变更。
type StateEffect struct {
	Field     string `json:"field"`
	Operation string `json:"operation"`
	Value     any    `json:"value"`
}

// ApplyStateEffects 先在副本上完整校验和执行，全部成功后才写回原状态。
func ApplyStateEffects(state map[string]any, effects []StateEffect, specs map[string]FieldSpec) error {
	working := cloneState(state)
	for _, effect := range effects {
		spec, ok := specs[effect.Field]
		if !ok {
			return fmt.Errorf("state field is undeclared: %s", effect.Field)
		}
		if !spec.Writable {
			return fmt.Errorf("state field is read-only: %s", effect.Field)
		}
		if err := applyStateEffect(working, effect, spec); err != nil {
			return err
		}
	}
	for key := range state {
		delete(state, key)
	}
	for key, value := range working {
		state[key] = value
	}
	return nil
}

func applyStateEffect(state map[string]any, effect StateEffect, spec FieldSpec) error {
	op := strings.ToLower(strings.TrimSpace(effect.Operation))
	switch op {
	case "set", "update", "assign", "write", "replace", "modify", "put":
		value, err := normalizeFieldValue(effect.Value, spec)
		if err != nil {
			return fmt.Errorf("set %s: %w", effect.Field, err)
		}
		state[effect.Field] = value
		return nil
	case "increment", "decrement", "add", "plus", "inc", "increase", "minus", "sub", "dec", "decrease", "reduce":
		if spec.Type == "string_set" || spec.Type == "event_set" {
			// 如果集合类型收到了 add/append
			return applyAppendEffect(state, effect, spec)
		}
		if spec.Type != "integer" && spec.Type != "number" && spec.Type != "counter" {
			return fmt.Errorf("field %s does not support numeric operation", effect.Field)
		}
		amount, ok := numericValue(effect.Value)
		if !ok {
			return fmt.Errorf("effect value for %s is not numeric", effect.Field)
		}
		current := 0.0
		if existing, exists := state[effect.Field]; exists {
			var currentOK bool
			current, currentOK = numericValue(existing)
			if !currentOK {
				return fmt.Errorf("current value for %s is not numeric", effect.Field)
			}
		}
		if op == "decrement" || op == "minus" || op == "sub" || op == "dec" || op == "decrease" || op == "reduce" {
			current -= amount
		} else {
			current += amount
		}
		current = clampNumber(current, spec)
		if spec.Type == "integer" || spec.Type == "counter" {
			state[effect.Field] = int(current)
		} else {
			state[effect.Field] = current
		}
		return nil
	case "append", "push", "insert", "attach":
		return applyAppendEffect(state, effect, spec)
	default:
		// 默认兜底为 set
		value, err := normalizeFieldValue(effect.Value, spec)
		if err != nil {
			return fmt.Errorf("apply %s: %w", effect.Field, err)
		}
		state[effect.Field] = value
		return nil
	}
}

func applyAppendEffect(state map[string]any, effect StateEffect, spec FieldSpec) error {
	if spec.Type != "string_set" && spec.Type != "event_set" {
		return fmt.Errorf("field %s does not support append", effect.Field)
	}
	value, ok := effect.Value.(string)
	if !ok || value == "" {
		return fmt.Errorf("append value for %s must be a non-empty string", effect.Field)
	}
	items, _ := state[effect.Field].([]any)
	for _, item := range items {
		if item == value {
			return nil
		}
	}
	state[effect.Field] = append(items, value)
	return nil
}

func normalizeFieldValue(value any, spec FieldSpec) (any, error) {
	switch spec.Type {
	case "boolean":
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("expected boolean")
		}
		return value, nil
	case "integer", "counter":
		number, ok := numericValue(value)
		if !ok || number != float64(int(number)) {
			return nil, fmt.Errorf("expected integer")
		}
		return int(clampNumber(number, spec)), nil
	case "number":
		number, ok := numericValue(value)
		if !ok {
			return nil, fmt.Errorf("expected number")
		}
		return clampNumber(number, spec), nil
	case "enum":
		text, ok := value.(string)
		if !ok || !spec.Allowed[text] {
			return nil, fmt.Errorf("value is not an allowed enum")
		}
		return text, nil
	case "string":
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("expected string")
		}
		return value, nil
	default:
		return value, nil
	}
}

func clampNumber(value float64, spec FieldSpec) float64 {
	if spec.HasMin && value < spec.Min {
		value = spec.Min
	}
	if spec.HasMax && value > spec.Max {
		value = spec.Max
	}
	return value
}

func cloneState(state map[string]any) map[string]any {
	clone := make(map[string]any, len(state))
	for key, value := range state {
		if list, ok := value.([]any); ok {
			clone[key] = append([]any(nil), list...)
		} else {
			clone[key] = value
		}
	}
	return clone
}
