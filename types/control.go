package types

// ControlSignal 驱动调度器行为
type ControlSignal string

const (
	// NEXT 执行下一任务
	NEXT ControlSignal = "NEXT"
	// JUMP 跳转到指定节点
	JUMP ControlSignal = "JUMP"
	// REPEAT 重新执行当前节点
	REPEAT ControlSignal = "REPEAT"
	// STOP 停止执行
	STOP ControlSignal = "STOP"
)
