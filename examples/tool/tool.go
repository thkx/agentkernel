package examplestool

import (
	"fmt"

	"github.com/thkx/agentkernel/types"
)

var attemptCount = 0

type Tool struct{}

func (t *Tool) Name() types.CapabilityName { return "tool" }

func (t *Tool) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("examplestool(%v)", input), nil
}
