# flodk

A Go framework for building stateful, graph-based workflows with support for human-in-the-loop interrupts and LLM integration.

## Overview

flodk is a workflow orchestration framework that enables you to build complex, multi-step processes as directed graphs. It provides built-in support for:

- **Graph-based workflows**: Define nodes and edges to create complex execution flows
- **State management**: Thread-safe state persistence across workflow steps
- **Human-in-the-loop interrupts**: Pause execution and request user input with validation
- **LLM integration**: Built-in nodes for data extraction and processing using LLM providers
- **Checkpoint/resume**: Resume workflow execution from interruption points
- **Conditional routing**: Dynamic edge resolution based on state

## Architecture

### Core Components

- **Node**: The basic unit of work in a workflow. Executes logic and transforms state.
- **Graph**: A directed graph of nodes connected by edges that define execution flow.
- **Edge/EdgeResolver**: Determines the next node to execute based on current state.
- **Flow**: Manages the execution of a graph with callbacks at various stages.
- **Pipe**: Supervises graph execution, manages state persistence, and handles interrupts.
- **Store**: Persists execution state for resumption after interrupts.

### State Management

The framework uses a checkpoint-based approach to maintain execution state:

- **CheckpointState**: Stores the current node, visited nodes, and interrupt history
- **ExecutionState**: Combines checkpoint state with application-specific state
- **ExecutionID**: Uniquely identifies an execution (ID + flow name)

## Usage

### Basic Workflow

```go
import (
 "context"
 "github.com/aki-kong/flodk"
)

type MyState struct {
 Value string
}

func main() {
 ctx := context.Background()

 // Build the graph
 gb := flodk.NewGraphBuilder[MyState]()
 graph, _ := gb.
  AddNode("start", flodk.Noop[MyState]()).
  AddNode("end", flodk.Noop[MyState]()).
  AddEdge("start", "end").
  SetStartNode("start").
  Build()

 // Create a pipe with an in-memory store
 store := flodk.NewInMemoryStore[MyState]()
 pipe := flodk.NewPipe("my_workflow", graph, store)

 // Execute the workflow
 state := MyState{Value: "initial"}
 result, err := pipe.Invoke(ctx, "thread-123", state)
}
```

### Custom Nodes

Implement the Node interface to create custom processing steps:

```go
type MyNode struct{}

func (n MyNode) Execute(ctx context.Context, state MyState) (MyState, error) {
 state.Value = "processed"
 return state, nil
}
```

### Conditional Routing

Route execution based on state values:

```go
graph, _ := gb.
 AddConditionalEdge(
  "decision",
  flodk.ConditionalFunction[MyState](func(ctx context.Context, state MyState) string {
   if state.Value == "proceed" {
    return "next_step"
   }
   return "alternate_step"
  }),
  map[string]string{
   "next_step":      "next_step",
   "alternate_step": "alternate_step",
  },
 ).
 Build()
```

### Human-in-the-Loop Interrupts

Request user input during workflow execution:

```go
values, err := flodk.InterruptWithValidation(
 ctx,
 "Please provide your name",
 "name_required",
 flodk.Requirements{
  "name": {Type: flodk.Custom},
 },
 flodk.Requirements.Validate,
)

if err != nil {
 // Handle interrupt error
 // User will need to call pipe.Continue() with values
 return state, err
}

state.Name = values["name"]
```

### LLM Integration

Extract structured data using LLM providers:

```go
llmClient := ollama.NewOllamaClient(baseURL)

extractionNode := llm.NewDataExtraction[MyState](llmClient, "model-name").
 Extract("field1", llm.DTString).
 Extract("field2", llm.DTInteger)

graph, _ := gb.
 AddNode("extract", extractionNode).
 Build()
```

## Supported LLM Providers

- **Ollama**: Local LLM inference
- **Custom**: Implement the `llm.Client` interface

## Workflow Resumption

When an interrupt occurs, the workflow state is persisted. Resume execution with:

```go
state, err := pipe.Continue(ctx, "thread-123", flodk.ResumeConfig{
 InterruptValues: map[string]string{
  "name": "John Doe",
 },
})
```

## Example

See the `example/main.go` for a complete flight booking workflow that demonstrates:

- Data extraction with LLM
- User input collection with validation
- Conditional routing
- State persistence and resumption

## Installation

```bash
go get github.com/aki-kong/flodk
```

## License

Licensed under the Apache License, Version 2.0. See the LICENSE file for details.
