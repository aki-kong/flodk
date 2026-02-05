package flodk

import (
	"errors"
	"fmt"
	"maps"
	"os"
)

type Graph[T any] struct {
	nodeMap map[string]Node[T]
	edges   map[string]EdgeResolver[T]

	start string
}

type GraphBuilder[T any] struct {
	g Graph[T]
}

func NewGraphBuilder[T any]() *GraphBuilder[T] {
	return &GraphBuilder[T]{
		g: Graph[T]{
			nodeMap: make(map[string]Node[T]),
			edges:   make(map[string]EdgeResolver[T]),
		},
	}
}

// AddNode adds a node to the graph.
func (gb *GraphBuilder[T]) AddNode(name string, node Node[T]) *GraphBuilder[T] {
	gb.g.nodeMap[name] = node
	return gb
}

// AddNodes adds multiple named nodes to the graph.
func (gb *GraphBuilder[T]) AddNodes(nodes map[string]Node[T]) *GraphBuilder[T] {
	maps.Copy(gb.g.nodeMap, nodes)

	return gb
}

// AddEdge adds a single edge relation.
func (gb *GraphBuilder[T]) AddEdge(start, end string) *GraphBuilder[T] {
	if _, ok := gb.g.nodeMap[start]; !ok {
		fmt.Fprintf(os.Stderr, "start node not found: %s, skipping", start)
		return gb
	}

	if _, ok := gb.g.nodeMap[end]; !ok {
		fmt.Fprintf(os.Stderr, "end node not found: %s, skipping", start)
		return gb
	}

	gb.g.edges[start] = ConstEdge[T](end)

	return gb
}

// AddEdge adds a single edge relation with a conditional redirection.
func (gb *GraphBuilder[T]) AddConditionalEdge(start string, end ConditionalNode[T], redirections map[string]string) *GraphBuilder[T] {
	if _, ok := gb.g.nodeMap[start]; !ok {
		fmt.Fprintf(os.Stderr, "start node not found: %s, skipping", start)
		return gb
	}

	endNodes := map[string]string{}
	for k, v := range redirections {
		if _, ok := gb.g.nodeMap[v]; !ok {
			fmt.Fprintf(os.Stderr, "end node not found: %s, skipping", start)
			continue
		}

		endNodes[k] = v
	}

	gb.g.edges[start] = ConditionalEdge[T]{
		exec:         end,
		redirections: redirections,
	}

	return gb
}

// SetStartNode sets the start node of the graph.
func (gb *GraphBuilder[T]) SetStartNode(start string) *GraphBuilder[T] {
	if start == "" {
		fmt.Fprintf(os.Stderr, "start node cannot be empty: %s, skipping", start)
		return gb
	}

	if _, ok := gb.g.nodeMap[start]; !ok {
		fmt.Fprintf(os.Stderr, "start node not found: %s, skipping", start)
		return gb
	}

	gb.g.start = start
	return gb
}

// Build checks for the validity of the graph and returns the graph.
func (gb *GraphBuilder[T]) Build() (Graph[T], error) {
	if gb.g.start == "" {
		return Graph[T]{}, errors.New("no invocation node found")
	}

	// TODO: Test for circular deps while building the graph

	return gb.g, nil
}
