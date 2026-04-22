package policy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/thkx/agentkernel/types"
)

var methodCallRegex = regexp.MustCompile(`^(\w+)\.(\w+)\(([^)]+)\)$`)

// ExpressionEvaluator is the unified evaluator for conditions and inputs
type ExpressionEvaluator struct {
	conditionCache     map[string]*ConditionExpression
	variableExtractors map[string]VariableExtractor
}

// VariableExtractor defines how to extract values from custom sources
type VariableExtractor func(name string, context *EvalContext) interface{}

// EvalContext provides context for expression evaluation
type EvalContext struct {
	Variables map[string]interface{}
	State     map[string]interface{}
	Result    *types.Result
	InputData interface{}
	Metadata  map[string]interface{}
}

// NewExpressionEvaluator creates a new expression evaluator
func NewExpressionEvaluator() *ExpressionEvaluator {
	return &ExpressionEvaluator{
		conditionCache:     make(map[string]*ConditionExpression),
		variableExtractors: make(map[string]VariableExtractor),
	}
}

// RegisterVariableExtractor registers a custom variable extractor
func (ee *ExpressionEvaluator) RegisterVariableExtractor(name string, extractor VariableExtractor) {
	ee.variableExtractors[name] = extractor
}

// EvaluateCondition evaluates a condition expression against a result
func (ee *ExpressionEvaluator) EvaluateCondition(expression string, ctx *EvalContext) (bool, error) {
	if expression == "" || expression == "*" {
		return true, nil
	}

	// Check cache
	condExpr, exists := ee.conditionCache[expression]
	if !exists {
		parsed, err := ee.parseConditionExpression(expression)
		if err != nil {
			return false, err
		}
		condExpr = parsed
		ee.conditionCache[expression] = condExpr
	}

	return ee.evaluateCondition(condExpr, ctx), nil
}

// EvaluateInput evaluates input expressions with variable substitution
// Supports:
// - Static values: "hello", 123, true
// - Variable references: "$varName"
// - Template strings: "user: $user, age: $age"
func (ee *ExpressionEvaluator) EvaluateInput(input interface{}, ctx *EvalContext) (interface{}, error) {
	if input == nil {
		return nil, nil
	}

	// Handle string inputs with potential variable references
	if s, ok := input.(string); ok {
		// Simple variable reference: $varName
		if strings.HasPrefix(s, "$") {
			varName := s[1:]
			return ee.resolveVariable(varName, ctx)
		}

		// Template string with variables: "text $var text"
		if strings.Contains(s, "$") {
			return ee.evaluateTemplate(s, ctx)
		}
	}

	// Handle maps recursively
	if m, ok := input.(map[string]interface{}); ok {
		result := make(map[string]interface{})
		for k, v := range m {
			evaluated, err := ee.EvaluateInput(v, ctx)
			if err != nil {
				return nil, err
			}
			result[k] = evaluated
		}
		return result, nil
	}

	// Handle slices recursively
	if arr, ok := input.([]interface{}); ok {
		result := make([]interface{}, len(arr))
		for i, v := range arr {
			evaluated, err := ee.EvaluateInput(v, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = evaluated
		}
		return result, nil
	}

	return input, nil
}

// parseConditionExpression parses a condition expression string
func (ee *ExpressionEvaluator) parseConditionExpression(expr string) (*ConditionExpression, error) {
	expr = strings.TrimSpace(expr)

	var condExpr ConditionExpression
	condExpr.Expression = expr

	// Check for method call syntax: field.method(arg)
	if methodCallRegex.MatchString(expr) {
		matches := methodCallRegex.FindStringSubmatch(expr)
		if len(matches) == 4 {
			condExpr.IsMethodCall = true
			condExpr.Left = strings.TrimSpace(matches[1])   // field
			condExpr.Method = strings.TrimSpace(matches[2]) // method
			condExpr.Arg = strings.TrimSpace(matches[3])    // arg
			return &condExpr, nil
		}
	}

	// List of operators in order of precedence
	operators := []string{
		"==", "!=", "<=", ">=", "<", ">",
		"contains", "startswith", "endswith", "in",
	}

	for _, op := range operators {
		parts := strings.SplitN(expr, " "+op+" ", 2)
		if len(parts) == 2 {
			condExpr.Left = strings.TrimSpace(parts[0])
			condExpr.Operator = op
			condExpr.Right = strings.TrimSpace(parts[1])
			return &condExpr, nil
		}
	}

	return nil, fmt.Errorf("invalid condition expression: %s", expr)
}

// evaluateCondition evaluates a parsed condition
func (ee *ExpressionEvaluator) evaluateCondition(expr *ConditionExpression, ctx *EvalContext) bool {
	if expr.IsMethodCall {
		fieldVal := ee.extractFieldValue(expr.Left, ctx)
		argVal := ee.parseValue(expr.Arg, ctx)
		switch expr.Method {
		case "contains":
			return ee.contains(fieldVal, argVal)
		case "startswith":
			return ee.startsWith(fieldVal, argVal)
		case "endswith":
			return ee.endsWith(fieldVal, argVal)
		default:
			return false
		}
	}

	leftVal := ee.extractFieldValue(expr.Left, ctx)
	rightVal := ee.parseValue(expr.Right, ctx)

	switch expr.Operator {
	case "==":
		return ee.equal(leftVal, rightVal)
	case "!=":
		return !ee.equal(leftVal, rightVal)
	case "<":
		return ee.lessThan(leftVal, rightVal)
	case ">":
		return ee.greaterThan(leftVal, rightVal)
	case "<=":
		return ee.lessThanOrEqual(leftVal, rightVal)
	case ">=":
		return ee.greaterThanOrEqual(leftVal, rightVal)
	case "contains":
		return ee.contains(leftVal, rightVal)
	case "startswith":
		return ee.startsWith(leftVal, rightVal)
	case "endswith":
		return ee.endsWith(leftVal, rightVal)
	case "in":
		return ee.in(leftVal, rightVal)
	default:
		return false
	}
}

// extractFieldValue extracts a value from the context based on field name
func (ee *ExpressionEvaluator) extractFieldValue(field string, ctx *EvalContext) interface{} {
	field = strings.TrimSpace(field)

	// Handle variables first
	if strings.HasPrefix(field, "$") {
		varName := field[1:]
		result, _ := ee.resolveVariable(varName, ctx)
		return result
	}

	// Handle special result fields
	if ctx != nil && ctx.Result != nil {
		switch field {
		case "status":
			return string(ctx.Result.Status)
		case "output":
			return ctx.Result.Output
		case "error":
			if ctx.Result.Error != nil {
				return ctx.Result.Error.Error()
			}
			return ""
		case "control":
			return string(ctx.Result.Control)
		case "attempt":
			return ctx.Result.Attempt
		case "retryable":
			return ctx.Result.Retryable
		}
	}

	// Try to extract from result.Output if it's a map
	if ctx != nil && ctx.Result != nil {
		if m, ok := ctx.Result.Output.(map[string]interface{}); ok {
			if v, exists := m[field]; exists {
				return v
			}
		}
	}

	return nil
}

// resolveVariable resolves a variable from multiple sources
func (ee *ExpressionEvaluator) resolveVariable(varName string, ctx *EvalContext) (interface{}, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context not provided")
	}

	// Try custom extractors first
	for prefix, extractor := range ee.variableExtractors {
		if strings.HasPrefix(varName, prefix+".") {
			return extractor(varName, ctx), nil
		}
	}

	// Check explicit variables
	if v, exists := ctx.Variables[varName]; exists {
		return v, nil
	}

	// Check state
	if v, exists := ctx.State[varName]; exists {
		return v, nil
	}

	// Check metadata
	if v, exists := ctx.Metadata[varName]; exists {
		return v, nil
	}

	return nil, fmt.Errorf("variable %q not found", varName)
}

// evaluateTemplate evaluates a template string with variable substitution
func (ee *ExpressionEvaluator) evaluateTemplate(template string, ctx *EvalContext) (string, error) {
	re := regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_.]*)`)

	var lastErr error
	result := re.ReplaceAllStringFunc(template, func(match string) string {
		varName := match[1:] // Remove '$' prefix
		val, err := ee.resolveVariable(varName, ctx)
		if err != nil {
			lastErr = err
			return match
		}
		return ee.toString(val)
	})

	if lastErr != nil {
		return "", lastErr
	}

	return result, nil
}

// parseValue parses a value from a string representation
func (ee *ExpressionEvaluator) parseValue(val interface{}, ctx *EvalContext) interface{} {
	if s, ok := val.(string); ok {
		s = strings.TrimSpace(s)

		// Check if it's a variable reference
		if strings.HasPrefix(s, "$") {
			result, _ := ee.resolveVariable(s[1:], ctx)
			return result
		}

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
				arr[i] = ee.parseValue(strings.TrimSpace(item), ctx)
			}
			return arr
		}

		return s
	}
	return val
}

// Comparison functions
func (ee *ExpressionEvaluator) equal(left, right interface{}) bool {
	return ee.compareValues(left, right) == 0
}

func (ee *ExpressionEvaluator) lessThan(left, right interface{}) bool {
	return ee.compareValues(left, right) < 0
}

func (ee *ExpressionEvaluator) greaterThan(left, right interface{}) bool {
	return ee.compareValues(left, right) > 0
}

func (ee *ExpressionEvaluator) lessThanOrEqual(left, right interface{}) bool {
	return ee.compareValues(left, right) <= 0
}

func (ee *ExpressionEvaluator) greaterThanOrEqual(left, right interface{}) bool {
	return ee.compareValues(left, right) >= 0
}

// compareValues compares two values and returns -1, 0, or 1
func (ee *ExpressionEvaluator) compareValues(left, right interface{}) int {
	leftStr := ee.toString(left)
	rightStr := ee.toString(right)

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
func (ee *ExpressionEvaluator) contains(haystack, needle interface{}) bool {
	h := ee.toString(haystack)
	n := ee.toString(needle)
	return strings.Contains(h, n)
}

func (ee *ExpressionEvaluator) startsWith(text, prefix interface{}) bool {
	t := ee.toString(text)
	p := ee.toString(prefix)
	return strings.HasPrefix(t, p)
}

func (ee *ExpressionEvaluator) endsWith(text, suffix interface{}) bool {
	t := ee.toString(text)
	s := ee.toString(suffix)
	return strings.HasSuffix(t, s)
}

func (ee *ExpressionEvaluator) in(item interface{}, list interface{}) bool {
	switch arr := list.(type) {
	case []interface{}:
		for _, v := range arr {
			if ee.equal(item, v) {
				return true
			}
		}
	case []string:
		itemStr := ee.toString(item)
		for _, v := range arr {
			if v == itemStr {
				return true
			}
		}
	}
	return false
}

// toString converts any value to string
func (ee *ExpressionEvaluator) toString(v interface{}) string {
	if v == nil {
		return ""
	}
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
