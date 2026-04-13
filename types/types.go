package types

type ControlSignal string

const (
	NEXT  ControlSignal = "NEXT"
	STOP  ControlSignal = "STOP"
	RETRY ControlSignal = "RETRY"
)

type ExecStatus string

const (
	SUCCESS ExecStatus = "SUCCESS"
	FAILED  ExecStatus = "FAILED"
)

type Task struct {
	ID      string
	Type    string
	Input   any
	Attempt int
}

type Result struct {
	Output    any
	Status    ExecStatus
	Control   ControlSignal
	Error     error
	NextNode  string
	Retryable bool
}

type ExecContext struct {
	TaskID string
	State  map[string]any
}
