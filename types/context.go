package types

type ExecContext struct {
	TraceID string
	SpanID  string
	NodeID  NodeID
	State   map[string]any
}

type HookContext struct {
	TraceID    string
	SpanID     string
	NodeID     NodeID
	Capability CapabilityName
	Input      any
	Attempt    int
	State      map[string]any
	Result     Result
}

type HookFunc func(HookContext)
