package main

import (
	"time"

	"github.com/thkx/agentkernel/runtime"
)

func main() {
	rt := runtime.NewRuntime()

	rt.Start("hello agent v0.1")

	time.Sleep(2 * time.Second)
}
