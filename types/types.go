package types

type ExecStatus string

const (
	SUCCESS ExecStatus = "SUCCESS"
	FAILED  ExecStatus = "FAILED"
)

type TaskStatus string

const (
	Pending TaskStatus = "PENDING"
	Running TaskStatus = "RUNNING"
	Done    TaskStatus = "DONE"
	Failed  TaskStatus = "FAILED"
)

type Task struct {
	ID             string
	Type           string
	Input          any
	Attempt        int
	Status         TaskStatus
	IdempotencyKey string
	NodeID         NodeID
}
