package flodk

import (
	"context"
	"testing"
)

type State struct {
	sum int
}

type AdderNode int

func (a AdderNode) Execute(ctx context.Context, state State) (State, error) {
	state.sum += int(a)
	return state, nil
}

type GtNode int

const (
	Continue = "continue"
	End      = "End"
)

func (gn GtNode) Execute(ctx context.Context, state State) string {
	if state.sum >= int(gn) {
		return End
	}

	return Continue
}

func TestGraph(t *testing.T) {
	state := State{sum: 6}

	gb := NewGraphBuilder[State]()
	node, err := gb.
		AddNode("addition_1", AdderNode(1)).
		AddNode("addition_2", AdderNode(2)).
		AddNode("end", Noop[State]()).
		AddEdge("addition_1", "addition_2").
		AddConditionalEdge("addition_2", GtNode(10), map[string]string{
			Continue: "addition_1",
			End:      "end",
		}).SetStartNode("addition_1").Build()

	if err != nil {
		t.Errorf("error while building the graph: %s", err)
		return
	}

	flow := NewFlow("number_play", node)

	final, err := flow.Execute(t.Context(), state)
	if err != nil {
		t.Errorf("error while executing the graph nodes: %s", err)
		return
	}

	t.Logf("Final State: %+v\n\n", final)
}
