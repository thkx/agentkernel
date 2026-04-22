package types

type Result struct {
	Output    any
	Status    ExecStatus
	Control   ControlSignal
	NextNode  NodeID
	Error     error
	Attempt   int
	Retryable bool
}
