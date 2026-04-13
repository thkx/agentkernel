package types

type ControlSignal string

const (
	NEXT  ControlSignal = "NEXT"
	JUMP  ControlSignal = "JUMP"
	RETRY ControlSignal = "RETRY"
	STOP  ControlSignal = "STOP"
)
