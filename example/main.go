package main

import (
	"context"
	"encoding/json"
	"errors"
	"flodk"
	"fmt"
	"os"
	"time"
)

type FlightBookingState struct {
	Name        string
	Origin      string
	Destination string
	Flights     []string
}

type Greet string

func (g Greet) Execute(ctx context.Context, state FlightBookingState) (FlightBookingState, error) {
	requirements := flodk.Requirements{
		"name": {
			Type: flodk.Custom,
		},
	}
	values, err := flodk.InterruptWithValidation(ctx,
		"Before we continue, may I know your name?",
		"name_not_found",
		requirements,
		requirements.Validate,
	)
	if err != nil {
		return state, err
	}

	name := values["name"]

	fmt.Printf("👋 %s, %s!!\n", g, name)
	state.Name = name

	return state, nil
}

func enc() *json.Encoder {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	return enc
}

type Gather struct{}

func (g Gather) Execute(ctx context.Context, state FlightBookingState) (FlightBookingState, error) {
	requirements := flodk.Requirements{
		"origin": {
			Type: flodk.Custom,
		},
		"destination": {
			Type: flodk.Custom,
		},
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

	state.Origin = origin
	state.Destination = dest

	fmt.Printf("✈️ Finding flights from %s to %s!!\n", origin, dest)
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

	ctx := context.Background()
	gb := flodk.NewGraphBuilder[FlightBookingState]()
	graph, err := gb.
		AddNode("greet", Greet("Hola")).
		AddNode("get_details", Gather{}).
		AddNode("end", flodk.Noop[FlightBookingState]()).
		AddEdge("greet", "get_details").
		AddEdge("get_details", "end").
		SetStartNode("greet").Build()
	if err != nil {
		panic(err)
	}

	store := flodk.NewInMemoryStore[FlightBookingState]()

	pipe := flodk.NewPipe("book_flights", graph, store)

	state := FlightBookingState{}
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
