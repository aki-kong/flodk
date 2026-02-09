package flodk

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
)

// Graph stores the graph nodes and edge configuration.
type Graph[T any] struct {
	nodeMap map[string]Node[T]
	edges   map[string]EdgeResolver[T]

	start string
}

// GraphBuilder is a helper type which contains methods to build a graph.
type GraphBuilder[T any] struct {
	g Graph[T]
}

// NewGraphBuilder returns a new GraphBuilder. Chain this return with other methods to [GraphBuilder.Build]
// a graph.
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

	// Check that at least one terminal node (no outgoing edge) is reachable
	// from the start. If not, the graph will loop forever.
	if !hasReachableTerminal(gb.g.start, gb.g.edges) {
		return Graph[T]{}, errors.New("graph has no reachable terminal node from start: execution would loop forever")
	}

	return gb.g, nil
}

// hasReachableTerminal performs a DFS from start and returns true if at least
// one node with no outgoing edge is reachable.
func hasReachableTerminal[T any](start string, edges map[string]EdgeResolver[T]) bool {
	visited := map[string]bool{}

	var dfs func(node string) bool
	dfs = func(node string) bool {
		if visited[node] {
			return false
		}
		visited[node] = true

		resolver, hasEdge := edges[node]
		if !hasEdge {
			// Terminal node found.
			return true
		}

		return slices.ContainsFunc(resolver.edges(), dfs)
	}

	return dfs(start)
}
