package capability

import "github.com/thkx/agentkernel/types"

type Capability interface {
	Name() string
	Invoke(ctx types.ExecContext, input any) (any, error)
}
