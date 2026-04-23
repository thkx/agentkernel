package policy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/thkx/agentkernel/types"
)

var methodCallRegexEval = regexp.MustCompile(`^(\w+)\.(\w+)\(([^)]+)\)$`)

// ConditionEvaluator evaluates condition expressions
type ConditionEvaluator struct {
	// Cache parsed conditions
	cache map[string]*ConditionExpression
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator() *ConditionEvaluator {
	return &ConditionEvaluator{
		cache: make(map[string]*ConditionExpression),
	}
}

// BuildCondition builds a condition function from a string expression
// Supports simple expressions like:
//   - "status == 'success'"
//   - "output.contains('yes')"
//   - "result > 100"
//   - "code in ['200', '201']"
func (ce *ConditionEvaluator) BuildCondition(expr string) (func(types.Result) bool, error) {
	if expr == "" || expr == "*" {
		// Always true condition
		return func(r types.Result) bool { return true }, nil
	}

	// Parse the expression
	condExpr, err := ce.parseExpression(expr)
	if err != nil {
		return nil, err
	}

	// Build evaluator function
	return func(result types.Result) bool {
		return ce.evaluateExpression(condExpr, result)
	}, nil
}

// parseExpression parses a condition expression string
func (ce *ConditionEvaluator) parseExpression(expr string) (*ConditionExpression, error) {
	expr = strings.TrimSpace(expr)

	// Check cache
	if cached, exists := ce.cache[expr]; exists {
		return cached, nil
	}

	var condExpr ConditionExpression
	condExpr.Expression = expr

	// Check for method call syntax: field.method(arg)
	if methodCallRegexEval.MatchString(expr) {
		matches := methodCallRegexEval.FindStringSubmatch(expr)
		if len(matches) == 4 {
			condExpr.IsMethodCall = true
			condExpr.Left = strings.TrimSpace(matches[1])   // field
			condExpr.Method = strings.TrimSpace(matches[2]) // method
			condExpr.Arg = strings.TrimSpace(matches[3])    // arg
			ce.cache[expr] = &condExpr
			return &condExpr, nil
		}
	}

	// Try to match a simple binary expression: "left op right"
	// Operators: ==, !=, <, >, <=, >=, contains, startswith, endswith, in
	operators := []string{
		"==", "!=", "<=", ">=",
		"<", ">",
		"contains", "startswith", "endswith",
		"in",
	}

	for _, op := range operators {
		parts := strings.SplitN(expr, " "+op+" ", 2)
		if len(parts) == 2 {
			condExpr.Left = strings.TrimSpace(parts[0])
			condExpr.Operator = op
			condExpr.Right = strings.TrimSpace(parts[1])

			ce.cache[expr] = &condExpr
			return &condExpr, nil
		}
	}

	return nil, fmt.Errorf("invalid condition expression: %s", expr)
}

// evaluateExpression evaluates a parsed expression against a result
func (ce *ConditionEvaluator) evaluateExpression(expr *ConditionExpression, result types.Result) bool {
	if expr.IsMethodCall {
		fieldVal := ce.extractValue(expr.Left, result)
		argVal := ce.parseValue(expr.Arg)
		switch expr.Method {
		case "contains":
			return ce.contains(fieldVal, argVal)
		case "startswith":
			return ce.startsWith(fieldVal, argVal)
		case "endswith":
			return ce.endsWith(fieldVal, argVal)
		default:
			return false
		}
	}

	leftVal := ce.extractValue(expr.Left, result)
	rightVal := ce.parseValue(expr.Right)

	switch expr.Operator {
	case "==":
		return ce.equal(leftVal, rightVal)
	case "!=":
		return !ce.equal(leftVal, rightVal)
	case "<":
		return ce.lessThan(leftVal, rightVal)
	case ">":
		return ce.greaterThan(leftVal, rightVal)
	case "<=":
		return ce.lessThanOrEqual(leftVal, rightVal)
	case ">=":
		return ce.greaterThanOrEqual(leftVal, rightVal)
	case "contains":
		return ce.contains(leftVal, rightVal)
	case "startswith":
		return ce.startsWith(leftVal, rightVal)
	case "endswith":
		return ce.endsWith(leftVal, rightVal)
	case "in":
		return ce.in(leftVal, rightVal)
	default:
		return false
	}
}

// extractValue extracts a value from the result based on the field name
func (ce *ConditionEvaluator) extractValue(field string, result types.Result) interface{} {
	field = strings.TrimSpace(field)

	// Handle special fields
	switch field {
	case "status":
		return string(result.Status)
	case "output":
		return result.Output
	case "error":
		if result.Error != nil {
			return result.Error.Error()
		}
		return ""
	case "control":
		return string(result.Control)
	case "attempt":
		return result.Attempt
	case "retryable":
		return result.Retryable
	default:
		// Try to extract from result.Output if it's a map
		if m, ok := result.Output.(map[string]interface{}); ok {
			if v, exists := m[field]; exists {
				return v
			}
		}
		return result.Output
	}
}

// parseValue parses a value from a string representation
func (ce *ConditionEvaluator) parseValue(val interface{}) interface{} {
	if s, ok := val.(string); ok {
		s = strings.TrimSpace(s)

		// Remove quotes if present
		if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
			(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
			return s[1 : len(s)-1]
		}

		// Try to parse as number
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}

		// Try to parse as boolean
		if s == "true" {
			return true
		}
		if s == "false" {
			return false
		}

		// Try to parse as array (for "in" operator)
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			content := s[1 : len(s)-1]
			items := strings.Split(content, ",")
			arr := make([]interface{}, len(items))
			for i, item := range items {
				arr[i] = ce.parseValue(strings.TrimSpace(item))
			}
			return arr
		}

		return s
	}
	return val
}

// Comparison functions
func (ce *ConditionEvaluator) equal(left, right interface{}) bool {
	return ce.compareValues(left, right) == 0
}

func (ce *ConditionEvaluator) lessThan(left, right interface{}) bool {
	return ce.compareValues(left, right) < 0
}

func (ce *ConditionEvaluator) greaterThan(left, right interface{}) bool {
	return ce.compareValues(left, right) > 0
}

func (ce *ConditionEvaluator) lessThanOrEqual(left, right interface{}) bool {
	return ce.compareValues(left, right) <= 0
}

func (ce *ConditionEvaluator) greaterThanOrEqual(left, right interface{}) bool {
	return ce.compareValues(left, right) >= 0
}

// compareValues compares two values and returns -1, 0, or 1
func (ce *ConditionEvaluator) compareValues(left, right interface{}) int {
	// Convert to comparable types
	leftStr := ce.toString(left)
	rightStr := ce.toString(right)

	// Try numeric comparison first
	if leftNum, leftErr := strconv.ParseFloat(leftStr, 64); leftErr == nil {
		if rightNum, rightErr := strconv.ParseFloat(rightStr, 64); rightErr == nil {
			if leftNum < rightNum {
				return -1
			} else if leftNum > rightNum {
				return 1
			}
			return 0
		}
	}

	// Fall back to string comparison
	if leftStr < rightStr {
		return -1
	} else if leftStr > rightStr {
		return 1
	}
	return 0
}

// String operations
func (ce *ConditionEvaluator) contains(haystack, needle interface{}) bool {
	h := ce.toString(haystack)
	n := ce.toString(needle)
	return strings.Contains(h, n)
}

func (ce *ConditionEvaluator) startsWith(text, prefix interface{}) bool {
	t := ce.toString(text)
	p := ce.toString(prefix)
	return strings.HasPrefix(t, p)
}

func (ce *ConditionEvaluator) endsWith(text, suffix interface{}) bool {
	t := ce.toString(text)
	s := ce.toString(suffix)
	return strings.HasSuffix(t, s)
}

func (ce *ConditionEvaluator) in(item interface{}, list interface{}) bool {
	// If list is an array
	switch arr := list.(type) {
	case []interface{}:
		for _, v := range arr {
			if ce.equal(item, v) {
				return true
			}
		}
	case []string:
		itemStr := ce.toString(item)
		for _, v := range arr {
			if v == itemStr {
				return true
			}
		}
	}
	return false
}

// toString converts any value to string
func (ce *ConditionEvaluator) toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int32, int64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%f", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// InputEvaluator evaluates input expressions and variables
type InputEvaluator struct {
	variables map[string]interface{}
}

// NewInputEvaluator creates a new input evaluator
func NewInputEvaluator(variables map[string]interface{}) *InputEvaluator {
	return &InputEvaluator{
		variables: variables,
	}
}

// EvaluateInput evaluates input which may contain variable references
// Supports:
//   - Static values: "hello", 123, true
//   - Variable references: "$varName"
//   - Complex templates: "user: $user, age: $age"
func (ie *InputEvaluator) EvaluateInput(input interface{}, stateStore map[string]interface{}) (interface{}, error) {
	if input == nil {
		return nil, nil
	}

	// If it's a string, check for variable references
	if s, ok := input.(string); ok {
		// Check if it's a simple variable reference
		if strings.HasPrefix(s, "$") {
			varName := s[1:]
			// First check local variables
			if v, exists := ie.variables[varName]; exists {
				return v, nil
			}
			// Then check state store
			if v, exists := stateStore[varName]; exists {
				return v, nil
			}
			return nil, fmt.Errorf("variable not found: %s", varName)
		}

		// Check for template variables (e.g., "user: $user")
		if strings.Contains(s, "$") {
			return ie.evaluateTemplate(s, stateStore)
		}
	}

	// Return as-is for other types
	return input, nil
}

// evaluateTemplate evaluates a template string with variables
func (ie *InputEvaluator) evaluateTemplate(template string, stateStore map[string]interface{}) (string, error) {
	// Simple regex-based variable substitution
	re := regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)

	result := re.ReplaceAllStringFunc(template, func(match string) string {
		varName := match[1:] // Remove '$' prefix
		if v, exists := ie.variables[varName]; exists {
			return fmt.Sprintf("%v", v)
		}
		if v, exists := stateStore[varName]; exists {
			return fmt.Sprintf("%v", v)
		}
		return match // Return unchanged if variable not found
	})

	return result, nil
}
