package flodk

import "context"

// Node represents any node of the execution graph.
type Node[T any] interface {
	// Execute runs the node logic with the passed in state.
	Execute(ctx context.Context, state T) (T, error)
}

type noop[T any] struct{}

func (n noop[T]) Execute(ctx context.Context, state T) (T, error) {
	return state, nil
}

func Noop[T any]() Node[T] {
	return noop[T]{}
}

type ConditionalNode[T any] interface {
	Execute(ctx context.Context, state T) string
}
