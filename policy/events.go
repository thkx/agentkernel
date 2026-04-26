package policy

import (
	"time"

	"github.com/thkx/agentkernel/types"
)

func publishPolicyEvent(bus types.EventBus, name, policyName, source string, valid bool, nodeCount int, err error, warnings, errors int, metadata map[string]any) {
	if bus == nil {
		return
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	payload := types.PolicyEvent{
		Name:       name,
		PolicyName: policyName,
		Source:     source,
		Valid:      valid,
		NodeCount:  nodeCount,
		Error:      errMsg,
		Warnings:   warnings,
		Errors:     errors,
		Metadata:   metadata,
		Timestamp:  time.Now(),
	}

	bus.Publish(types.Event{
		Kind:      types.EventKindPolicy,
		Name:      name,
		Timestamp: payload.Timestamp,
		Result:    payload,
	})
}

func summarizeValidation(result *ValidationResult) (warnings int, errors int) {
	if result == nil {
		return 0, 0
	}
	for _, item := range result.Errors {
		if item.Level == "warning" {
			warnings++
			continue
		}
		if item.Level == "error" {
			errors++
		}
	}
	return warnings, errors
}
