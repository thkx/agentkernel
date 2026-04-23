package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/observability"
	"github.com/thkx/agentkernel/retry"
	"github.com/thkx/agentkernel/state"
	"github.com/thkx/agentkernel/types"
	"github.com/thkx/agentkernel/worker"
)

// Scheduler is the main execution engine
type Scheduler struct {
	engine         *GraphEngine[any]
	caps           *capability.Registry
	event          types.EventBus
	taskStates     map[types.NodeID]types.TaskStatus
	executed       map[string]bool
	dlq            *DeadLetterQueue
	retryPol       *retry.RetryPolicy
	metrics        types.MetricsRecorder
	beforeHooks    []types.HookFunc
	afterHooks     []types.HookFunc
	beforeWritable []types.WritableHookFunc
	afterWritable  []types.WritableHookFunc
	workerCount    int
	defaultTimeout time.Duration
	tenantAware    bool
	queue          *taskQueue
	delayedQueue   *delayedTaskQueue
	flowCtl        *flowController
	queued         map[types.NodeID]bool
	inFlight       int
	signalCh       chan struct{}
	mu             sync.Mutex
}

func New(engine *GraphEngine[any], bus types.EventBus, caps *capability.Registry, opts ...SchedulerOption) *Scheduler {
	s := &Scheduler{
		engine:         engine,
		caps:           caps,
		event:          bus,
		taskStates:     make(map[types.NodeID]types.TaskStatus),
		executed:       make(map[string]bool),
		dlq:            NewDeadLetterQueue(),
		retryPol:       retry.NewRetryPolicy(3, 100*time.Millisecond),
		metrics:        observability.NewMetrics(),
		beforeHooks:    make([]types.HookFunc, 0),
		afterHooks:     make([]types.HookFunc, 0),
		beforeWritable: make([]types.WritableHookFunc, 0),
		afterWritable:  make([]types.WritableHookFunc, 0),
		workerCount:    10,
		defaultTimeout: 3 * time.Second,
		tenantAware:    true,
		queue:          newTaskQueue(100, RejectPolicyWait),
		delayedQueue:   newDelayedTaskQueue(),
		flowCtl:        newFlowController(),
		queued:         make(map[types.NodeID]bool),
		signalCh:       make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Scheduler) Register(cap types.Capability) {
	s.caps.Register(cap)
}

func (s *Scheduler) RegisterBeforeHook(hook types.HookFunc) {
	s.beforeHooks = append(s.beforeHooks, hook)
}

func (s *Scheduler) RegisterAfterHook(hook types.HookFunc) {
	s.afterHooks = append(s.afterHooks, hook)
}

func (s *Scheduler) RegisterBeforeWritableHook(hook types.WritableHookFunc) {
	s.beforeWritable = append(s.beforeWritable, hook)
}

func (s *Scheduler) RegisterAfterWritableHook(hook types.WritableHookFunc) {
	s.afterWritable = append(s.afterWritable, hook)
}

func (s *Scheduler) Metrics() types.MetricsRecorder {
	return s.metrics
}

func (s *Scheduler) DLQSize() int {
	return s.dlq.Size()
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *Scheduler) enqueueTask(nodeID types.NodeID) bool {
	node := s.engine.GetNode(nodeID)
	if node == nil {
		return false
	}

	s.mu.Lock()
	prevStatus, hadPrevStatus := s.taskStates[nodeID]
	if s.queued[nodeID] || prevStatus == types.Running || prevStatus == types.Done {
		s.mu.Unlock()
		return false
	}
	s.queued[nodeID] = true
	s.mu.Unlock()

	item := &taskItem{
		nodeID:    nodeID,
		priority:  node.Priority,
		tenant:    node.Tenant,
		createdAt: time.Now(),
	}

	result := s.queue.enqueue(item, s.tenantAware)
	if result.dropped {
		log.Printf("level=warn msg=task_rejected node=%s policy=%d", nodeID, s.queue.rejectionPol)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if result.success {
		s.taskStates[nodeID] = types.Pending
		s.signal()
		return true
	}
	delete(s.queued, nodeID)
	if hadPrevStatus {
		s.taskStates[nodeID] = prevStatus
	} else {
		delete(s.taskStates, nodeID)
	}
	return result.success
}

func (s *Scheduler) popNextTask() *taskItem {
	item := s.queue.dequeue(s.tenantAware)
	if item != nil {
		s.mu.Lock()
		delete(s.queued, item.nodeID)
		s.mu.Unlock()
	}
	return item
}

func (s *Scheduler) canProceedWithRateLimit(node *types.Node[any]) bool {
	return s.flowCtl.canProceedWithTenant(node)
}

func (s *Scheduler) isCircuitBreakerOpen(node *types.Node[any]) bool {
	return s.flowCtl.isOpen(node)
}

func (s *Scheduler) requeueAfter(nodeID types.NodeID, delay time.Duration) {
	s.mu.Lock()
	s.delayedQueue.push(nodeID, time.Now().Add(delay))
	s.mu.Unlock()
	s.signal()
}

func (s *Scheduler) invokeCapabilityWithTimeout(cap types.Capability, ctx types.ExecContext, input any, timeout time.Duration) (any, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx.Ctx, timeout)
	defer cancel()

	ctx.Ctx = ctxWithTimeout
	return cap.Invoke(ctx, input)
}

func executionKey(node *types.Node[any]) string {
	return string(node.ID) + ":" + string(node.Capability)
}

func (s *Scheduler) isAlreadyExecuted(nodeID types.NodeID) bool {
	node := s.engine.GetNode(nodeID)
	if node == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.executed[executionKey(node)]
}

func (s *Scheduler) Run(start types.NodeID, state map[string]any) {
	s.RunWithContext(context.Background(), start, state)
}

func (s *Scheduler) RunWithContext(ctx context.Context, start types.NodeID, state map[string]any) {
	graph := s.engine.graph
	if graph == nil {
		log.Println("level=error msg=graph_nil")
		return
	}

	// Crash recovery: replay events to restore state
	s.resetRunState()
	s.recoverState()
	runState := statepkg(state)

	startNode := start
	if startNode == "" {
		startNode = graph.Start
	}
	reachable := s.buildReachableSet(graph, startNode)
	dependencies, children := s.buildExecutionPlan(graph, reachable)
	pendingDeps := make(map[types.NodeID]int)
	for id := range reachable {
		count := 0
		for _, dep := range dependencies[id] {
			if !s.isAlreadyExecuted(dep) {
				count++
			}
		}
		pendingDeps[id] = count
	}

	remaining := 0
	for id := range reachable {
		if !s.isAlreadyExecuted(id) {
			remaining++
		}
	}

	for id, count := range pendingDeps {
		if count == 0 && !s.isAlreadyExecuted(id) {
			s.enqueueTask(id)
		}
	}

	var wg sync.WaitGroup
	workerPool := worker.NewPool(s.workerCount)
	defer workerPool.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dispatchDone := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "dispatch goroutine panicked: %v\n", r)
			}
			close(dispatchDone)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			s.releaseReadyDelayedTasks()

			item := s.popNextTask()
			if item == nil {
				if s.shouldStopDispatch(&remaining) {
					return
				}
				s.waitForWork(ctx)
				continue
			}

			node := s.engine.GetNode(item.nodeID)
			if node == nil {
				continue
			}

			if !s.canProceedWithRateLimit(node) {
				s.requeueAfter(item.nodeID, 100*time.Millisecond)
				continue
			}

			s.incrementInFlight()
			wg.Add(1)
			workerPool.Submit(func() {
				defer wg.Done()
				defer s.decrementInFlight()
				s.executeNodeWithContext(ctx, node, runState, children, pendingDeps, &remaining, cancel)
			})
		}
	}()

	wg.Wait()
	<-dispatchDone
}

func (s *Scheduler) executeNode(node *types.Node[any], runState *state.Store, children map[types.NodeID][]types.Edge, pendingDeps map[types.NodeID]int, remaining *int) {
	s.executeNodeWithContext(context.Background(), node, runState, children, pendingDeps, remaining, func() {})
}

func (s *Scheduler) executeNodeWithContext(ctx context.Context, node *types.Node[any], runState *state.Store, children map[types.NodeID][]types.Edge, pendingDeps map[types.NodeID]int, remaining *int, cancel context.CancelFunc) {
	traceID := generateID()
	spanID := generateID()
	key := executionKey(node)

	s.mu.Lock()
	if s.executed[key] || s.taskStates[node.ID] == types.Running {
		s.mu.Unlock()
		log.Printf("level=info msg=skip_already_executed node=%s trace=%s span=%s", node.ID, traceID, spanID)
		return
	}
	s.taskStates[node.ID] = types.Running
	s.mu.Unlock()

	for _, hook := range s.beforeHooks {
		hook(types.NewHookContext(traceID, spanID, node.ID, node.Capability, node.Input, 0, s.hookStateAccessor(runState, false), types.Result{}))
	}
	for _, hook := range s.beforeWritable {
		hook(types.NewWritableHookContext(traceID, spanID, node.ID, node.Capability, node.Input, 0, runState, types.Result{}))
	}

	cap, ok := s.caps.Get(node.Capability)
	if !ok {
		log.Printf("level=error msg=capability_not_found node=%s capability=%s trace=%s span=%s", node.ID, node.Capability, traceID, spanID)
		s.mu.Lock()
		s.taskStates[node.ID] = types.Failed
		s.mu.Unlock()
		result := types.Result{Output: nil, Error: fmt.Errorf("capability_not_found"), Status: types.FAILED, Control: types.NEXT}
		s.handleFailure(node, traceID, spanID, runState, result)
		s.processDependents(node.ID, children, pendingDeps, remaining, result, cancel)
		return
	}

	if s.isCircuitBreakerOpen(node) {
		result := types.Result{Output: nil, Error: fmt.Errorf("circuit_breaker_open"), Status: types.FAILED, Control: types.NEXT}
		s.handleFailure(node, traceID, spanID, runState, result)
		s.processDependents(node.ID, children, pendingDeps, remaining, result, cancel)
		return
	}

	timeout := node.Timeout
	if timeout <= 0 {
		timeout = s.defaultTimeout
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()
	execCtx := types.NewExecContext(ctxWithTimeout, traceID, spanID, node.ID, runState)
	output, err := cap.Invoke(execCtx, node.Input)
	status := types.SUCCESS
	if err != nil {
		status = types.FAILED
	}

	result := types.Result{Output: output, Error: err, Status: status, Control: types.NEXT}

	attempt := 1
	if err != nil {
		var retryCount int
		var retryOK bool
		ctxWithRetryTimeout, retryCancel := context.WithTimeout(ctx, timeout)
		defer retryCancel()
		execCtxRetry := types.NewExecContext(ctxWithRetryTimeout, traceID, spanID, node.ID, runState)
		retryOK, retryCount = s.retryNode(node.ID, node, execCtxRetry, cap, &result)
		attempt = retryCount
		if retryOK {
			s.mu.Lock()
			s.taskStates[node.ID] = types.Done
			s.executed[key] = true
			s.mu.Unlock()
			s.flowCtl.recordResult(node, true)
		} else {
			s.mu.Lock()
			s.taskStates[node.ID] = types.Failed
			s.mu.Unlock()
			task := types.Task{ID: string(node.ID), Type: string(node.Capability), Input: node.Input, Attempt: s.retryPol.MaxAttempts, Status: types.Failed, IdempotencyKey: key, NodeID: node.ID}
			s.dlq.Enqueue(task)
			s.flowCtl.recordResult(node, false)
		}
	} else {
		s.mu.Lock()
		s.taskStates[node.ID] = types.Done
		s.executed[key] = true
		s.mu.Unlock()
		s.flowCtl.recordResult(node, true)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	s.metrics.Record(duration)

	timeline := types.ExecutionTimelineEntry{TraceID: traceID, SpanID: spanID, NodeID: node.ID, Capability: node.Capability, Status: result.Status, StartTime: startTime, EndTime: endTime, Duration: duration, Attempt: attempt, Error: fmt.Sprintf("%v", result.Error)}
	s.event.Store().AppendTimeline(timeline)
	s.event.Publish(types.Event{NodeID: string(node.ID), TraceID: traceID, SpanID: spanID, Result: result})

	for _, hook := range s.afterHooks {
		hook(types.NewHookContext(traceID, spanID, node.ID, node.Capability, node.Input, attempt, s.hookStateAccessor(runState, false), result))
	}
	for _, hook := range s.afterWritable {
		hook(types.NewWritableHookContext(traceID, spanID, node.ID, node.Capability, node.Input, attempt, runState, result))
	}

	log.Printf("level=info msg=exec_finished node=%s capability=%s trace=%s span=%s status=%s latency_ms=%d", node.ID, node.Capability, traceID, spanID, result.Status, duration.Milliseconds())

	s.processDependents(node.ID, children, pendingDeps, remaining, result, cancel)
}

func (s *Scheduler) handleFailure(node *types.Node[any], traceID, spanID string, runState *state.Store, result types.Result) {
	for _, hook := range s.afterHooks {
		hook(types.NewHookContext(traceID, spanID, node.ID, node.Capability, node.Input, result.Attempt, s.hookStateAccessor(runState, false), result))
	}
	for _, hook := range s.afterWritable {
		hook(types.NewWritableHookContext(traceID, spanID, node.ID, node.Capability, node.Input, result.Attempt, runState, result))
	}
}

func (s *Scheduler) processDependents(nodeID types.NodeID, children map[types.NodeID][]types.Edge, pendingDeps map[types.NodeID]int, remaining *int, result types.Result, cancel context.CancelFunc) {
	// Handle control flow
	if result.Control == types.STOP {
		s.mu.Lock()
		*remaining = 0
		s.mu.Unlock()
		s.queue.signal()
		cancel()
		s.signal()
		return
	}

	if result.Control == types.JUMP && result.NextNode != "" {
		// Jump to specific node, bypassing normal dependents
		if !s.isAlreadyExecuted(result.NextNode) {
			s.enqueueTask(result.NextNode)
		}
		s.mu.Lock()
		*remaining--
		s.mu.Unlock()
		s.queue.signal()
		s.signal()
		return
	}

	if result.Control == types.REPEAT {
		s.enqueueTask(nodeID)
		s.queue.signal()
		s.signal()
		return
	}

	var toEnqueue []types.NodeID
	s.mu.Lock()
	for _, edge := range children[nodeID] {
		if edge.Condition != nil && !edge.Condition(result) {
			continue
		}
		childNode := s.engine.GetNode(edge.To)
		if childNode == nil {
			continue
		}
		pendingDeps[edge.To]--
		if pendingDeps[edge.To] == 0 && !s.executed[executionKey(childNode)] {
			toEnqueue = append(toEnqueue, edge.To)
		}
	}
	*remaining--
	s.mu.Unlock()
	for _, childID := range toEnqueue {
		s.enqueueTask(childID)
	}
	s.queue.signal()
	s.signal()
}

func (s *Scheduler) recoverState() {
	s.event.Replay(func(ev types.Event) {
		if result, ok := ev.Result.(types.Result); ok {
			nodeID := types.NodeID(ev.NodeID)
			s.mu.Lock()
			defer s.mu.Unlock()
			if result.Status == types.SUCCESS {
				s.taskStates[nodeID] = types.Done
				if node := s.engine.GetNode(nodeID); node != nil {
					s.executed[executionKey(node)] = true
				}
			} else {
				s.taskStates[nodeID] = types.Failed
			}
		}
	})
}

func (s *Scheduler) buildExecutionPlan(graph *types.Graph[any], reachable map[types.NodeID]struct{}) (map[types.NodeID][]types.NodeID, map[types.NodeID][]types.Edge) {
	deps := make(map[types.NodeID][]types.NodeID)
	children := make(map[types.NodeID][]types.Edge)
	for id, node := range graph.Nodes {
		if len(reachable) > 0 {
			if _, ok := reachable[id]; !ok {
				continue
			}
		}
		for _, edge := range node.Next {
			if len(reachable) > 0 {
				if _, ok := reachable[edge.To]; !ok {
					continue
				}
			}
			deps[edge.To] = append(deps[edge.To], id)
			children[id] = append(children[id], edge)
		}
	}
	return deps, children
}

func (s *Scheduler) buildReachableSet(graph *types.Graph[any], start types.NodeID) map[types.NodeID]struct{} {
	reachable := make(map[types.NodeID]struct{})
	if start == "" {
		return reachable
	}
	if _, ok := graph.Nodes[start]; !ok {
		return reachable
	}

	stack := []types.NodeID{start}
	for len(stack) > 0 {
		last := len(stack) - 1
		nodeID := stack[last]
		stack = stack[:last]
		if _, seen := reachable[nodeID]; seen {
			continue
		}
		reachable[nodeID] = struct{}{}
		node := graph.Nodes[nodeID]
		if node == nil {
			continue
		}
		for _, edge := range node.Next {
			if _, ok := graph.Nodes[edge.To]; ok {
				stack = append(stack, edge.To)
			}
		}
	}

	return reachable
}

func (s *Scheduler) retryNode(nodeID types.NodeID, node *types.Node[any], ctx types.ExecContext, cap capability.Capability, result *types.Result) (bool, int) {
	for attempt := 2; s.retryPol.ShouldRetry(attempt); attempt++ {
		log.Printf("level=warn msg=retrying node=%s attempt=%d", nodeID, attempt)
		timer := time.NewTimer(s.retryPol.Next(attempt - 1))
		select {
		case <-ctx.Ctx.Done():
			timer.Stop()
			result.Error = ctx.Ctx.Err()
			result.Status = types.FAILED
			return false, attempt - 1
		case <-timer.C:
		}

		output, err := cap.Invoke(ctx, node.Input)
		if err == nil {
			result.Output = output
			result.Error = nil
			result.Status = types.SUCCESS
			return true, attempt
		}
		result.Error = err
	}
	return false, s.retryPol.MaxAttempts
}

func (s *Scheduler) shouldStopDispatch(remaining *int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	currentRemaining := *remaining
	return currentRemaining == 0 || (currentRemaining > 0 && s.inFlight == 0 && !s.hasQueuedTasksLocked() && s.delayedQueue.len() == 0)
}

func (s *Scheduler) incrementInFlight() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight++
}

func (s *Scheduler) decrementInFlight() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight > 0 {
		s.inFlight--
	}
	s.queue.signal()
	s.signal()
}

func (s *Scheduler) hasQueuedTasksLocked() bool {
	return len(s.queued) > 0
}

func (s *Scheduler) releaseReadyDelayedTasks() {
	now := time.Now()
	for {
		s.mu.Lock()
		nodeID, ok := s.delayedQueue.popReady(now)
		s.mu.Unlock()
		if !ok {
			return
		}
		s.enqueueTask(nodeID)
	}
}

func (s *Scheduler) waitForWork(ctx context.Context) {
	s.mu.Lock()
	nextReadyAt, hasDelayed := s.delayedQueue.nextReadyAt()
	s.mu.Unlock()

	if !hasDelayed {
		select {
		case <-ctx.Done():
		case <-s.signalCh:
		}
		return
	}

	delay := time.Until(nextReadyAt)
	if delay <= 0 {
		return
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-s.signalCh:
	case <-timer.C:
	}
}

func (s *Scheduler) signal() {
	select {
	case s.signalCh <- struct{}{}:
	default:
	}
}

func (s *Scheduler) resetRunState() {
	s.mu.Lock()
	s.taskStates = make(map[types.NodeID]types.TaskStatus)
	s.executed = make(map[string]bool)
	s.queued = make(map[types.NodeID]bool)
	s.inFlight = 0
	s.delayedQueue.reset()
	s.mu.Unlock()

	s.queue.reset()
	for {
		select {
		case <-s.signalCh:
		default:
			return
		}
	}
}

func statepkg(data map[string]any) *state.Store {
	if data == nil {
		return state.NewStore()
	}
	return state.NewStoreFromMap(data)
}

func (s *Scheduler) hookStateAccessor(runState *state.Store, writable bool) types.ReadOnlyStateAccessor {
	if runState == nil {
		return nil
	}
	if writable {
		return runState
	}
	return readOnlyStateView{store: runState}
}
