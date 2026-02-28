package flodk

import (
	"context"
	"errors"
)

// FlowCallback is a helper type which will be called during flow execution
// during different steps.
type FlowCallback[T any] func(cs CheckpointState, runState T)

// Call is a helper method which checks if the flow function passed is not nil
// and calls the same.
func (fc FlowCallback[T]) Call(cs CheckpointState, runState T) {
	if fc == nil {
		return
	}

	fc(cs, runState)
}

// Flow is a construct used start or resume execution of a graph with the
// passed initial app and checkpoint state.
type Flow[T any] struct {
	name      string
	graph     Graph[T]
	execState CheckpointState

	onNodeExecution  FlowCallback[T]
	onNodeResolution FlowCallback[T]
	onGraphEnd       FlowCallback[T]
}

// NewFlow create a new flow construct for the passed graph and name.
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

// WithCheckpoint is used to set the checkpoint state for this flow execution.
func (f *Flow[T]) WithCheckpoint(cp CheckpointState) *Flow[T] {
	f.execState = cp

	return f
}

// OnNodeExec sets the callback function to be called after a node is executed.
func (f *Flow[T]) OnNodeExec(cb FlowCallback[T]) *Flow[T] {
	f.onNodeExecution = cb

	return f
}

// OnNodeResolution sets the callback function to be called after the next node is resolved.
func (f *Flow[T]) OnNodeResolution(cb FlowCallback[T]) *Flow[T] {
	f.onNodeResolution = cb

	return f
}

// OnGraphEnd sets the callback function to be called after the graph execution is completed.
func (f *Flow[T]) OnGraphEnd(cb FlowCallback[T]) *Flow[T] {
	f.onGraphEnd = cb

	return f
}

// Execute executes the graph with provided initial state and resumes based on the passed
// checkpoint state configuration.
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

		if f.execState.Interrupt.InterruptID.NodeID == currentID {
			// If the current node successfully processed the interrupt,
			// then the interrupt will be pushed into resolved HITL slice
			// of the execState. The current interrupt will be reset.
			lint, ok := getLoadedInterrupt(ctx, f.execState.Interrupt)
			if ok {
				f.execState.InterruptHistory = append(
					f.execState.InterruptHistory,
					lint,
				)
			}

			f.execState.Interrupt = HITLInterrupt{}
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

// Name returns the name of the flow.
func (f *Flow[T]) Name() string {
	return f.name
}

// LoadNodeID is used to store the current graph node id (node name) into the passed context.
func LoadNodeID(ctx context.Context, nodeID string) context.Context {
	return context.WithValue(ctx, "current_node", nodeID)
}

// GetNodeID is used to retrieve the current node id (node name) from the context.
func GetNodeID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value("current_node").(string)
	return val, ok
}
