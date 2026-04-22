package types

import "time"

type NodeID string

type CapabilityName string

type Node[T any] struct {
	ID                NodeID
	Capability        CapabilityName
	Input             T
	Next              []Edge
	Priority          int
	Tenant            string
	Timeout           time.Duration
	RateLimit         int
	CircuitBreakerKey string
}

type Edge struct {
	To        NodeID
	Condition func(result Result) bool
}

type Graph[T any] struct {
	Start NodeID
	Nodes map[NodeID]*Node[T]
}

type ExecutionTimelineEntry struct {
	TraceID    string
	SpanID     string
	NodeID     NodeID
	Capability CapabilityName
	Status     ExecStatus
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Attempt    int
	Error      string
}

func NewGraph[T any](start NodeID, nodes map[NodeID]*Node[T]) *Graph[T] {
	return &Graph[T]{
		Start: start,
		Nodes: nodes,
	}
}
