package flodk

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

// Pipe is a graph execution supervisor which loads the necessary
// data from the checkpoint store and validates any HITL responses
// during resumption, after which it executes the flow with the
// right context.
type Pipe[T any] struct {
	name  string
	graph Graph[T]
	store Store[T]
}

// NewPipe creates a new Pipe state for the passed flow name, graph
// and store implementation.
func NewPipe[T any](
	name string,
	graph Graph[T],
	store Store[T],
) *Pipe[T] {
	return &Pipe[T]{
		name:  name,
		graph: graph,
		store: store,
	}
}

// persistStateFunc generates a generic callback function which Flow can call
// during each part of the execution.
func (p *Pipe[T]) persistStateFunc(ctx context.Context, id string) FlowCallback[T] {
	return func(cs CheckpointState, runState T) {
		// TODO: Handle error
		p.store.Set(ctx, ExecutionID{
			ID:       id,
			FlowName: p.name,
		}, ExecutionState[T]{
			CheckpointState:  cs,
			ApplicationState: runState,
		})
	}
}

// invoke is a common function which all the pipe execution functions use to
// start the flow execution. This takes in a unique identifier, checkpoint
// of the flow execution and execution's the initial state.
func (p *Pipe[T]) invoke(
	ctx context.Context,
	id string,
	checkpointState CheckpointState,
	initState T,
) (T, error) {
	storeFunc := p.persistStateFunc(ctx, id)
	flow := NewFlow(p.name, p.graph).
		WithCheckpoint(checkpointState).
		OnNodeExec(storeFunc).
		OnNodeResolution(storeFunc).
		OnGraphEnd(storeFunc)

	return flow.Execute(ctx, initState)
}

// Invoke is used to start a flow execution for a given unique identifier
// with the passed initial state.
func (p *Pipe[T]) Invoke(
	ctx context.Context,
	id string,
	initState T,
) (T, error) {
	return p.invoke(ctx, id, CheckpointState{
		Visited:          make([]string, 0),
		InterruptHistory: make([]ResolvedHITLInterrupt, 0),
	}, initState)
}

// ResumeConfig defines the values required for resuming the flow execution.
type ResumeConfig struct {
	// InterruptValues stores answer values provided during the HITL interration
	// for the HITL Interrupt in the previous execution step.
	InterruptValues map[string]string
}

// Continue is used to continue the flow execution right after interrupt. This method fetches
// the execution state for this flow (flow name) and the provided ID, validates the interrupt
// values provided against the original interrupt requirements.
func (p *Pipe[T]) Continue(
	ctx context.Context,
	id string,
	rc ResumeConfig,
) (T, error) {
	// Get the execution state for the passed ID and flow name.
	execState, err := p.store.Get(ctx, ExecutionID{
		ID:       id,
		FlowName: p.name,
	})
	if err != nil {
		return execState.ApplicationState, err
	}

	// Validate the interrupt values and collect the interrupt values
	interruptValues := make(map[string]string, len(execState.CheckpointState.Interrupt.Requirements))
	for key, req := range execState.CheckpointState.Interrupt.Requirements {
		ans, ok := rc.InterruptValues[key]
		if !ok {
			return execState.ApplicationState, RequirementKeyNotFound(key)
		}

		if req.Type == Enum && !slices.Contains(req.Suggestions, ans) {
			return execState.ApplicationState, RequirementInvalid(key, ans, req.Suggestions)
		}

		interruptValues[key] = ans
	}

	// Load the context with the interrupt values.
	loadedCtx := LoadInterrupt(
		ctx,
		execState.CheckpointState.Interrupt,
		interruptValues,
	)

	// Resume the flow processing with the checkpoint execution state, app state
	// interrupt values stored in the flow execution context.
	return p.invoke(loadedCtx, id, execState.CheckpointState, execState.ApplicationState)
}

// LoadInterrupt is used to load the context with a resolved interrupt (original HITLInterrupt and answer values).
func LoadInterrupt(ctx context.Context, interrupt HITLInterrupt, values map[string]string) context.Context {
	return context.WithValue(ctx, "interrupt_of:"+interrupt.InterruptID.NodeID, ResolvedHITLInterrupt{
		HITLInterrupt: interrupt,
		Values:        values,
	})
}

// getLoadedInterrupt is a private function used to just get the loaded resolved interrupt from the context.
func getLoadedInterrupt(ctx context.Context, interrupt HITLInterrupt) (ResolvedHITLInterrupt, bool) {
	rint, ok := ctx.Value("interrupt_of:" + interrupt.InterruptID.NodeID).(ResolvedHITLInterrupt)
	return rint, ok
}

// Interrupt is a helper function which calls [InterruptWithValidation] with a no validation.
func Interrupt(
	ctx context.Context,
	message string,
	reason string,
	values Requirements,
) (map[string]string, error) {
	return InterruptWithValidation(ctx, message, reason, values, func(map[string]string) error {
		return nil
	})
}

// InterruptWithValidation is used to test and return if the execution context contains any resolved interrupts
// or return a newly created interrupt. This also validates the resolved values passed for the interrupt in the context.
//
// Usage:
// When a node calls this function for the first time (read: no HITL is present for a graph nodeID)
// a new interrupt is created and returned as a error.
// When the smae node calls this function again (read: a resolved HITL interrupt is possibly stored in the context)
// the previously created HITL interrupt is found with resolved answer from the user.
//
// Validation:
// When the resolved interrupt is found, the values sumbitted by the user is passed through the validation function.
// Any error returned will be bubbled up as a HITL Interrupt with a validation error attached to it.
func InterruptWithValidation(
	ctx context.Context,
	message string,
	reason string,
	values Requirements,
	fn func(map[string]string) error,
) (map[string]string, error) {
	// Get the nodeID from the ctx. If a node is executed by a flow,
	// this value is always guaranteed to be set.
	nodeID, ok := GetNodeID(ctx)
	if !ok {
		// If this method is called using a context which is not
		// correctly initialized by the flow.Execute method, this error
		// will occur.
		return nil, errors.New("nodeID not found in context")
	}

	// Get the existing interrupt with resolved values if any.
	existingInterrupt, ok := ctx.Value("interrupt_of:" + nodeID).(ResolvedHITLInterrupt)
	if ok {
		// Validate the values
		err := fn(existingInterrupt.Values)
		if err != nil {
			existingInterrupt.HITLInterrupt.ValidationError = err
			return nil, existingInterrupt.HITLInterrupt
		}
		// Return the values.
		return existingInterrupt.Values, nil
	}

	// No HITL found? create a new interrupt.
	return nil, HITLInterrupt{
		Reason:       reason,
		Message:      message,
		Requirements: values,
		InterruptID: InterruptID{
			NodeID: nodeID,
			ID:     fmt.Sprintf("%x.%x", time.Now().UnixNano(), rand.Int64()),
		},
	}
}
