package worker

import "github.com/thkx/agentkernel/types"

type Worker struct {
	ID int
}

func (w *Worker) Do(task types.Task) types.Result {
	return types.Result{
		Output:  task.Input,
		Status:  types.SUCCESS,
		Control: types.NEXT,
	}
}
