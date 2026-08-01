package service

import (
	"fmt"
	"reflect"
	"strings"
)

// Condition 是 Manifest 中的单个确定性条件。
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

// ConditionGroup 支持 all/any/none 三种组合。
type ConditionGroup struct {
	All  []Condition `json:"all,omitempty"`
	Any  []Condition `json:"any,omitempty"`
	None []Condition `json:"none,omitempty"`
}

// EvaluateConditionGroup 只读取状态，不产生任何副作用。
func EvaluateConditionGroup(group ConditionGroup, state map[string]any) (bool, error) {
	for _, condition := range group.All {
		matched, err := evaluateCondition(condition, state)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}

	if len(group.Any) > 0 {
		matchedAny := false
		for _, condition := range group.Any {
			matched, err := evaluateCondition(condition, state)
			if err != nil {
				return false, err
			}
			if matched {
				matchedAny = true
				break
			}
		}
		if !matchedAny {
			return false, nil
		}
	}

	for _, condition := range group.None {
		matched, err := evaluateCondition(condition, state)
		if err != nil {
			return false, err
		}
		if matched {
			return false, nil
		}
	}
	return true, nil
}

func evaluateCondition(condition Condition, state map[string]any) (bool, error) {
	field := strings.TrimSpace(condition.Field)
	if field == "" {
		return false, fmt.Errorf("condition field is empty")
	}

	actual, exists := state[field]
	switch condition.Operator {
	case "exists":
		return exists, nil
	case "not_exists":
		return !exists, nil
	case "eq", "neq":
		if !exists {
			return condition.Operator == "neq", nil
		}
		equal := valuesEqual(actual, condition.Value)
		if condition.Operator == "neq" {
			return !equal, nil
		}
		return equal, nil
	case "gt", "gte", "lt", "lte":
		if !exists {
			return false, nil
		}
		left, ok := numericValue(actual)
		if !ok {
			return false, fmt.Errorf("condition field %s is not numeric", field)
		}
		right, ok := numericValue(condition.Value)
		if !ok {
			return false, fmt.Errorf("condition value for %s is not numeric", field)
		}
		switch condition.Operator {
		case "gt":
			return left > right, nil
		case "gte":
			return left >= right, nil
		case "lt":
			return left < right, nil
		default:
			return left <= right, nil
		}
	case "contains":
		if !exists {
			return false, nil
		}
		return containsValue(actual, condition.Value), nil
	default:
		return false, fmt.Errorf("unsupported condition operator: %s", condition.Operator)
	}
}

func valuesEqual(left, right any) bool {
	leftNumber, leftOK := numericValue(left)
	rightNumber, rightOK := numericValue(right)
	if leftOK && rightOK {
		return leftNumber == rightNumber
	}
	return reflect.DeepEqual(left, right)
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case float32:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func containsValue(container, wanted any) bool {
	switch value := container.(type) {
	case string:
		needle, ok := wanted.(string)
		return ok && strings.Contains(value, needle)
	case []any:
		for _, item := range value {
			if valuesEqual(item, wanted) {
				return true
			}
		}
	}
	return false
}
