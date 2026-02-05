package flodk

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

type Pipe[T any] struct {
	name  string
	graph Graph[T]
	store Store[T]
}

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

type ResumeConfig struct {
	InterruptValues map[string]string
}

func (p *Pipe[T]) Continue(
	ctx context.Context,
	id string,
	rc ResumeConfig,
) (T, error) {
	execState, err := p.store.Get(ctx, ExecutionID{
		ID:       id,
		FlowName: p.name,
	})
	if err != nil {
		return execState.ApplicationState, err
	}

	interruptValues := make(map[string]string, len(execState.CheckpointState.Interrupt.Requirements))
	for key, req := range execState.CheckpointState.Interrupt.Requirements {
		ans, ok := rc.InterruptValues[key]
		if !ok {
			// TODO: Use specific error types
			return execState.ApplicationState, fmt.Errorf("keys %s not found", key)
		}

		if req.Type == Enum && !slices.Contains(req.Suggestions, ans) {
			// TODO: Use specific error types
			return execState.ApplicationState, fmt.Errorf("invalid value for %s: %s, need one of %v", key, ans, req.Suggestions)
		}

		interruptValues[key] = ans
	}

	loadedCtx := LoadInterrupt(
		ctx,
		execState.CheckpointState.Interrupt,
		interruptValues,
	)

	return p.invoke(loadedCtx, id, execState.CheckpointState, execState.ApplicationState)
}

func LoadInterrupt(ctx context.Context, interrupt HITLInterrupt, values map[string]string) context.Context {
	return context.WithValue(ctx, "interrupt_of:"+interrupt.InterruptID.NodeID, ResolvedHITLInterrupt{
		HITLInterrupt: interrupt,
		Values:        values,
	})
}

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

func InterruptWithValidation(
	ctx context.Context,
	message string,
	reason string,
	values Requirements,
	fn func(map[string]string) error,
) (map[string]string, error) {
	nodeID, ok := GetNodeID(ctx)
	if !ok {
		// Possible tampering of execution order
		return nil, errors.New("nodeID not found in context")
	}

	existingInterrupt, ok := ctx.Value("interrupt_of:" + nodeID).(ResolvedHITLInterrupt)
	if ok {
		err := fn(existingInterrupt.Values)
		if err != nil {
			existingInterrupt.HITLInterrupt.ValidationError = err
			return nil, existingInterrupt.HITLInterrupt
		}
		return existingInterrupt.Values, nil
	}

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
