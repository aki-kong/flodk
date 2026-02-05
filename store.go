package flodk

import (
	"context"
)

type Store[T any] interface {
	Get(ctx context.Context, id ExecutionID) (ExecutionState[T], error)
	Set(ctx context.Context, id ExecutionID, state ExecutionState[T]) error
}

type InMemoryStore[T any] struct {
	states map[string]ExecutionState[T]
}

func NewInMemoryStore[T any]() *InMemoryStore[T] {
	return &InMemoryStore[T]{
		states: make(map[string]ExecutionState[T]),
	}
}

func (s *InMemoryStore[T]) Get(ctx context.Context, id ExecutionID) (ExecutionState[T], error) {
	key := id.ID + ":" + id.FlowName
	state, ok := s.states[key]
	if !ok {
		var zero ExecutionState[T]
		return zero, nil
	}
	return state, nil
}

func (s *InMemoryStore[T]) Set(ctx context.Context, id ExecutionID, state ExecutionState[T]) error {
	key := id.ID + ":" + id.FlowName
	s.states[key] = state
	return nil
}

type ExecutionID struct {
	ID       string `json:"id"`
	FlowName string `json:"flow_name"`
}

type ExecutionState[T any] struct {
	CheckpointState  CheckpointState `json:"checkpoint_state"`
	ApplicationState T               `json:"application_state"`
}

type CheckpointState struct {
	CheckpointID     string                  `json:"checkpoint_id"`
	Visited          []string                `json:"visited"`
	Interrupt        HITLInterrupt           `json:"interrupt"`
	InterruptHistory []ResolvedHITLInterrupt `json:"interrupt_history"`
}

type ResolvedHITLInterrupt struct {
	HITLInterrupt
	Values map[string]string
}
