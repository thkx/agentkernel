package main

import (
	"time"

	"github.com/thkx/agentkernel/runtime"
)

func main() {
	rt := runtime.NewRuntime()

	rt.Run()

	time.Sleep(2 * time.Second)
}
