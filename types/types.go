package types

import (
	"context"
	"time"
)

// ExecStatus represents execution status
type ExecStatus string

const (
	SUCCESS ExecStatus = "SUCCESS"
	FAILED  ExecStatus = "FAILED"
)

// TaskStatus represents task status
type TaskStatus string

const (
	Pending TaskStatus = "PENDING"
	Running TaskStatus = "RUNNING"
	Done    TaskStatus = "DONE"
	Failed  TaskStatus = "FAILED"
)

// Task represents a task
type Task struct {
	ID             string
	Type           string
	Input          any
	Attempt        int
	Status         TaskStatus
	IdempotencyKey string
	NodeID         NodeID
}

// Capability interface for capability abstraction
type Capability interface {
	Name() CapabilityName
	Invoke(ctx ExecContext, input any) (any, error)
}

type EventKind string

const (
	EventKindExecution EventKind = "EXECUTION"
	EventKindRuntime   EventKind = "RUNTIME"
	EventKindPolicy    EventKind = "POLICY"
)

// Event represents an event
type Event struct {
	Kind      EventKind
	Name      string
	NodeID    string
	TraceID   string
	SpanID    string
	Timestamp time.Time
	Result    any
}

type RuntimeLifecycleEvent struct {
	Name      string
	Status    string
	Previous  string
	Error     string
	Timestamp time.Time
}

type ExecutionEvent struct {
	Name       string
	NodeID     NodeID
	Capability CapabilityName
	TraceID    string
	SpanID     string
	Status     ExecStatus
	Attempt    int
	Duration   time.Duration
	Error      string
	Result     *Result
	Timestamp  time.Time
}

type PolicyEvent struct {
	Name        string
	PolicyName  string
	Source      string
	Valid       bool
	NodeCount   int
	Error       string
	Warnings    int
	Errors      int
	Metadata    map[string]any
	Timestamp   time.Time
}

// Scheduler interface for execution abstraction
type Scheduler interface {
	Run(start NodeID, state map[string]any)
	RunWithContext(ctx context.Context, start NodeID, state map[string]any)
	Register(cap Capability)
	RegisterBeforeHook(hook HookFunc)
	RegisterAfterHook(hook HookFunc)
	RegisterBeforeWritableHook(hook WritableHookFunc)
	RegisterAfterWritableHook(hook WritableHookFunc)
	Metrics() MetricsRecorder
	DLQSize() int
	StateSnapshot() map[string]any
	TaskStates() map[NodeID]TaskStatus
}

// EventBus interface for event abstraction
type EventBus interface {
	Publish(event Event)
	Subscribe() chan Event
	Unsubscribe(ch chan Event)
	Replay(handler func(Event)) error
	Store() EventStore
	Close() error
}

// EventStore interface for storage abstraction
type EventStore interface {
	Append(event Event) error
	Events() []Event
	Replay(handler func(Event)) error
	Snapshot() map[string]any
	AppendTimeline(entry ExecutionTimelineEntry) error
	Timeline() []ExecutionTimelineEntry
}

// MetricsRecorder interface for observability abstraction
type MetricsRecorder interface {
	Record(duration time.Duration)
	QPS() float64
	AvgLatency() time.Duration
	Fastest() time.Duration
	Slowest() time.Duration
	Total() int64
}

// Lifecycle interface for component management
type Lifecycle interface {
	Start() error
	Stop() error
}

// Configurable interface for component configuration
type Configurable[T any] interface {
	Configure(config T) error
}
