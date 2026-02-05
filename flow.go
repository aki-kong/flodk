package flodk

import (
	"context"
	"errors"
)

type FlowCallback[T any] func(cs CheckpointState, runState T)

func (fc FlowCallback[T]) Call(cs CheckpointState, runState T) {
	if fc == nil {
		return
	}

	fc(cs, runState)
}

type Flow[T any] struct {
	name      string
	graph     Graph[T]
	execState CheckpointState

	onNodeExecution  FlowCallback[T]
	onNodeResolution FlowCallback[T]
	onGraphEnd       FlowCallback[T]
}

func NewFlow[T any](
	name string,
	graph Graph[T],
) *Flow[T] {
	return &Flow[T]{
		name:      name,
		graph:     graph,
		execState: CheckpointState{},
	}
}

func (f *Flow[T]) WithCheckpoint(cp CheckpointState) *Flow[T] {
	f.execState = cp

	return f
}

func (f *Flow[T]) OnNodeExec(cb FlowCallback[T]) *Flow[T] {
	f.onNodeExecution = cb

	return f
}

func (f *Flow[T]) OnNodeResolution(cb FlowCallback[T]) *Flow[T] {
	f.onNodeResolution = cb

	return f
}

func (f *Flow[T]) OnGraphEnd(cb FlowCallback[T]) *Flow[T] {
	f.onGraphEnd = cb

	return f
}

func (f *Flow[T]) Execute(ctx context.Context, state T) (T, error) {
	currentID := f.graph.start
	if f.execState.CheckpointID != "" {
		currentID = f.execState.CheckpointID
	} else {
		// Set the current ID from the graph config
		f.execState.CheckpointID = currentID
	}

	runState := state

	// callback the state on function exit
	defer func() {
		f.onGraphEnd.Call(f.execState, runState)
	}()

	continueRunning := true

	for continueRunning {
		f.execState.Visited = append(f.execState.Visited, currentID)

		// Execute the current node.
		node := f.graph.nodeMap[currentID]
		currentState, err := node.Execute(LoadNodeID(ctx, currentID), runState)
		if err != nil {
			var interrupt HITLInterrupt
			if errors.As(err, &interrupt) {
				runState = currentState
				f.execState.Interrupt = interrupt
				continueRunning = false

				f.onNodeExecution.Call(f.execState, runState)
			}

			return runState, err
		}

		runState = currentState
		f.onNodeExecution.Call(f.execState, runState)

		// Resolve the next node.
		resolver, ok := f.graph.edges[currentID]
		if !ok {
			continueRunning = false
			continue
		}

		currentID = resolver.Resolve(ctx, runState)
		f.execState.CheckpointID = currentID
		f.onNodeResolution.Call(f.execState, runState)
	}

	return runState, nil
}

func (f *Flow[T]) Name() string {
	return f.name
}

func LoadNodeID(ctx context.Context, nodeID string) context.Context {
	return context.WithValue(ctx, "current_node", nodeID)
}

func GetNodeID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value("current_node").(string)
	return val, ok
}
