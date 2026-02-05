package flodk

import "context"

type EdgeResolver[T any] interface {
	Resolve(ctx context.Context, state T) string
}

type ConstEdge[T any] string

func (c ConstEdge[T]) Resolve(ctx context.Context, state T) string {
	return string(c)
}

type ConditionalEdge[T any] struct {
	exec         ConditionalNode[T]
	redirections map[string]string
}

func (ce ConditionalEdge[T]) Resolve(ctx context.Context, state T) string {
	next, ok := ce.redirections[ce.exec.Execute(ctx, state)]
	if !ok {
		return ""
	}

	return next
}
