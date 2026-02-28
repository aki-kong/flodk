package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aki-kong/flodk"
	"github.com/aki-kong/flodk/llm"
	"github.com/aki-kong/flodk/llm/ollama"
)

type FlightBookingState struct {
	RawPrompt   string   `json:"prompt"`
	Name        string   `json:"name" flodk_extraction:"username"`
	Origin      string   `json:"origin" flodk_extraction:"origin"`
	Destination string   `json:"destination" flodk_extraction:"destination"`
	Flights     []string `json:"flights"`
}

func (fbs FlightBookingState) Prompt() string {
	return fbs.RawPrompt
}

type Greet string

func (g Greet) Execute(ctx context.Context, state FlightBookingState) (FlightBookingState, error) {
	name := state.Name
	if name == "" {
		requirements := flodk.Requirements{
			"name": {
				Type: flodk.Custom,
			},
		}
		values, err := flodk.InterruptWithValidation(
			ctx,
			"Before we continue, may I know your name?",
			"name_not_found",
			requirements,
			requirements.Validate,
		)
		if err != nil {
			return state, err
		}

		name = values["name"]
		state.Name = name
	}

	fmt.Printf("👋 %s, %s!!\n", g, name)

	return state, nil
}

func enc() *json.Encoder {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	return enc
}

type Gather struct{}

func (g Gather) Execute(ctx context.Context, state FlightBookingState) (FlightBookingState, error) {
	requirements := flodk.Requirements{}

	if state.Origin == "" {
		requirements["origin"] = flodk.Requirement{
			Type: flodk.Custom,
		}
	}

	if state.Destination == "" {
		requirements["destination"] = flodk.Requirement{
			Type: flodk.Custom,
		}
	}

	values, err := flodk.InterruptWithValidation(ctx,
		"Please input your journey details",
		"journey_details_not_found",
		requirements,
		requirements.Validate,
	)
	if err != nil {
		return state, err
	}

	origin := values["origin"]
	dest := values["destination"]

	if state.Origin == "" {
		state.Origin = origin
	}

	if state.Destination == "" {
		state.Destination = dest
	}

	return state, nil
}

func findFlights(ctx context.Context, state FlightBookingState) (FlightBookingState, error) {
	fmt.Printf("✈️ Finding flights from %s to %s!!\n", state.Origin, state.Destination)
	for range 100 {
		fmt.Print(".")
		time.Sleep(16 * time.Millisecond)
	}

	state.Flights = []string{}
	fmt.Println("\n❌ No flights found")

	return state, nil
}

func main() {
	var err error

	prompt := strings.Join(os.Args[1:], " ")
	if prompt == "" {
		return
	}

	fmt.Printf("\033[1mPrompt\033[0m: %s\n", prompt)

	llmClient := ollama.NewOllamaClient("http://localhost:11434")
	nameExtractionNode := llm.NewDataExtraction[FlightBookingState](
		llmClient,
		os.Getenv("OLLAMA_MODEL"),
	).
		Extract("username", llm.DTString)

	routeExtractionNode := llm.NewDataExtraction[FlightBookingState](
		llmClient,
		os.Getenv("OLLAMA_MODEL"),
	).
		Extract("origin", llm.DTString).
		Extract("destination", llm.DTString)

	ctx := context.Background()
	gb := flodk.NewGraphBuilder[FlightBookingState]()
	graph, err := gb.
		AddNode("ai_greet", nameExtractionNode).
		AddNode("greet", Greet("Hola")).
		AddNode("ai_gather", routeExtractionNode).
		AddNode("manual_gather", Gather{}).
		AddNode("find_flights", flodk.FunctionNode[FlightBookingState](findFlights)).
		AddNode("end", flodk.Noop[FlightBookingState]()).
		AddEdge("ai_greet", "greet").
		AddEdge("greet", "ai_gather").
		AddConditionalEdge(
			"ai_gather",
			flodk.ConditionalFunction[FlightBookingState](func(ctx context.Context, state FlightBookingState) string {
				if state.Origin == "" || state.Destination == "" {
					return "manual_gather"
				}

				return "skip_manual_gather"
			}), map[string]string{
				"manual_gather":      "manual_gather",
				"skip_manual_gather": "find_flights",
			}).
		AddEdge("manual_gather", "find_flights").
		AddEdge("find_flights", "end").
		SetStartNode("ai_greet").Build()
	if err != nil {
		panic(err)
	}

	store := flodk.NewInMemoryStore[FlightBookingState]()

	pipe := flodk.NewPipe("book_flights", graph, store)

	state := FlightBookingState{RawPrompt: prompt}
	state, err = pipe.Invoke(ctx, "thread-123", state)
	if err == nil {
		fmt.Printf("State: %+v\n", state)
		return
	}

	hitl := flodk.HITLInterrupt{}
	for {
		if !errors.As(err, &hitl) {
			panic(err)
		}

		fmt.Println(hitl.Message)
		if hitl.ValidationError != nil {
			fmt.Printf("\033[31;1;4mValidation Failed:\033[0m %s\n", hitl.ValidationError)
		}

		hitlResp := map[string]string{}
		for k := range hitl.Requirements {
			val := ""
			fmt.Printf("Please input value for '%s': ", k)
			fmt.Scanf("%s", &val)

			hitlResp[k] = val
		}

		state, err = pipe.Continue(ctx, "thread-123", flodk.ResumeConfig{
			InterruptValues: hitlResp,
		})
		if err == nil {
			break
		}

	}

	enc().Encode(state)
}
