package types

type ExecContext struct {
	TraceID string
	NodeID  NodeID
	State   map[string]any
}
