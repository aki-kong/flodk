package flodk

import (
	"context"
)

// Store interface defines all the necessary functions used to store the execution and application state.
type Store[T any] interface {
	Get(ctx context.Context, id ExecutionID) (ExecutionState[T], error)
	Set(ctx context.Context, id ExecutionID, state ExecutionState[T]) error
}

// ExecutionID is a compound ID of a unique ID passed by the modules callers
// and name of the flow that is being executed.
type ExecutionID struct {
	ID       string `json:"id"`
	FlowName string `json:"flow_name"`
}

// ExecutionState stores the execution state [CheckpointState] and app state between executions.
type ExecutionState[T any] struct {
	CheckpointState  CheckpointState `json:"checkpoint_state"`
	ApplicationState T               `json:"application_state"`
}

// CheckpointState stores flow execution state which will be used to resume
// when the execution is interrupted.
type CheckpointState struct {
	// CheckpointID is the name of the graph node which will be picked up next when
	// the flow is executed.
	CheckpointID string `json:"checkpoint_id"`
	// Visited stores all the visited graph node (node IDs).
	Visited []string `json:"visited"`
	// Interrupt stores the Human in the loop interrupt when any node return a HITLInterrupt error.
	Interrupt HITLInterrupt `json:"interrupt"`
	// InterruptHistory stores all the resolved HITL interrupts.
	InterruptHistory []ResolvedHITLInterrupt `json:"interrupt_history"`
	// Execution Status of the graph.
	Status ExecutionStatus `json:"status"`
}

type ExecutionStatus string

const (
	ExecutionStatusInProgress  ExecutionStatus = "in-progress"
	ExecutionStatusInterrupted ExecutionStatus = "interrupted"
	ExecutionStatusEnd         ExecutionStatus = "ended"
)

// ResolvedHITLInterrupt contains the original HITL interrupt and the answer values submitted by the user.
type ResolvedHITLInterrupt struct {
	HITLInterrupt
	Values map[string]string `json:"values"`
}

// InMemoryStore implements the [Store] interface to store the checkpointing data in a in-memory map.
type InMemoryStore[T any] struct {
	// Disclaimer: This struct and it's methods are AI Generated, not the documentation.
	states map[string]ExecutionState[T]
}

// NewInMemoryStore create a new [InMemoryStore].
func NewInMemoryStore[T any]() *InMemoryStore[T] {
	return &InMemoryStore[T]{
		states: make(map[string]ExecutionState[T]),
	}
}

// Get implements the [Store.Get] method of the [Store] interface.
func (s *InMemoryStore[T]) Get(ctx context.Context, id ExecutionID) (ExecutionState[T], error) {
	key := id.ID + ":" + id.FlowName
	state, ok := s.states[key]
	if !ok {
		var zero ExecutionState[T]
		return zero, nil
	}
	return state, nil
}

// Set implements the [Store.Set] method of the [Store] interface.
func (s *InMemoryStore[T]) Set(ctx context.Context, id ExecutionID, state ExecutionState[T]) error {
	key := id.ID + ":" + id.FlowName
	s.states[key] = state
	return nil
}
