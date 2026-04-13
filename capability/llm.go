package capability

import (
	"fmt"

	"github.com/thkx/agentkernel/types"
)

type LLM struct{}

func (l *LLM) Name() string { return "llm" }

func (l *LLM) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("LLM(%s)", input), nil
}
