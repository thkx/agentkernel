package capability

import (
	"errors"
	"fmt"

	"github.com/thkx/agentkernel/types"
)

var attemptCount = 0

type Tool struct{}

func (t *Tool) Name() types.CapabilityName { return "tool" }

func (t *Tool) Invoke(ctx types.ExecContext, input any) (any, error) {
	attemptCount++
	if attemptCount == 1 {
		return nil, errors.New("tool failed on first attempt")
	}
	return fmt.Sprintf("TOOL(%v)", input), nil
}
