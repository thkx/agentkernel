package types

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
