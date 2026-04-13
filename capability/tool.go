package capability

import (
	"fmt"

	"github.com/thkx/agentkernel/types"
)

type Tool struct{}

func (t *Tool) Name() string { return "tool" }

func (t *Tool) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("TOOL(%v)", input), nil
}
