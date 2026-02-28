package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const (
	// defaultSystemPromptHeader defines the default system prompt sent to the LLMs
	// for data extraction use case.
	defaultSystemPromptHeader = `ABSOLUTELY NO MARKDOWN SYNTAX.
IF REQUIRED DATA IS NOT AVAILABLE IN THE HUMAN PROMPT, ALWAYS RETURN A EMPTY STRING AGAINST THAT FIELD.
For the given human prompt, your job is to understand the context
and extract the required data in single line raw JSON.`
)

// DataType type is used to define the data type of the data needed to be extracted from the
// User provided prompt.
type DataType string

const (
	DTInteger DataType = "integer"
	DTString  DataType = "string"
	DTBoolean DataType = "boolean"
	DTArray   DataType = "array"
	DTObject  DataType = "object"
)

// StateUpdateFunc is the signature of the state update function which will be used to update the state,
// for the extractedValues returned by the LLM.
type StateUpdateFunc[T any] func(currentState T, extractedValues map[string]any) (updatedState T)

// DataExtraction implements a Node using LLM which is used to extract
// specific data from the passed prompt.
type DataExtraction[T StateContract] struct {
	systemPromptHeader string
	model              string
	client             Client
	fields             map[string]DataType
	updateState        StateUpdateFunc[T]
}

// NewDataExtraction creates a LLM backed data extraction [flodk.Node]
func NewDataExtraction[T StateContract](
	client Client,
	model string,
) *DataExtraction[T] {
	return &DataExtraction[T]{
		systemPromptHeader: defaultSystemPromptHeader,
		model:              model,
		client:             client,
		fields:             make(map[string]DataType),
		updateState:        structTagKeySet[T],
	}
}

// Extract is a builder helper function used to specify the required parameters from the
// user provided prompt.
func (de *DataExtraction[T]) Extract(fieldName string, dt DataType) *DataExtraction[T] {
	de.fields[fieldName] = dt

	return de
}

// WithUpdateFunc is used to set a custom [StateUpdateFunc] once the required data is extracted by the LLM.
func (de *DataExtraction[T]) WithUpdateFunc(updateFunc StateUpdateFunc[T]) *DataExtraction[T] {
	de.updateState = updateFunc

	return de
}

// Execute implements the [flodk.Node] interface for DataExtraction.
func (de *DataExtraction[T]) Execute(ctx context.Context, state T) (T, error) {
	var sysPrompt strings.Builder
	sysPrompt.WriteString(de.systemPromptHeader + "\nFollowing Fields are needed:\n")

	properties := map[string]any{}
	for k, v := range de.fields {
		fmt.Fprintf(&sysPrompt, " - %s: %s\n", k, v)
		properties[k] = map[string]any{
			"type": v,
		}
	}

	jsonFormat, _ := json.Marshal(map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   []string{"name", "date", "amount"},
	})

	resp, err := de.client.GenerateContent(ctx, ChatRequest{
		Model: de.model,
		Messages: []Message{
			{"system", sysPrompt.String()},
			{"user", state.Prompt()},
		},
		Temperature: 0,
		Stream:      false,
		Format:      string(jsonFormat),
	})
	if err != nil {
		return state, err
	}

	if len(resp.Choices) < 1 {
		return state, errors.New("no choices in model response")
	}

	exValues := map[string]any{}

	err = json.NewDecoder(strings.NewReader(resp.Choices[0].Message.Content)).Decode(&exValues)
	if err != nil {
		return state, err
	}

	updateFunc := de.updateState
	if updateFunc == nil {
		// If updateState function is not set for some reason,
		// the default structTagKeySet function is used.
		//
		// Works well for scalar fields like strings, booleans and numbers.
		// For custom nested struct fields, better to write a custom [StateUpdateFunc] (Use [WithUpdateFunc]).
		updateFunc = structTagKeySet
	}

	return updateFunc(state, exValues), nil
}

// structTagKeySet is the default state update function which is used to set common simple values,
// like strings, numbers or booleans to the passed struct based on the `flodk_extraction` tag.
//
// Say a state struct is defined as the following:
//
//	type ExampleState struct {
//	    Prompt     string
//	    Username   string  `flodk_extraction:"username"`
//	}
//
// For a extracted map `{"username": "john"}`, the value `john` will be set as the Username.
func structTagKeySet[T any](currentState T, extractedValue map[string]any) T {
	newState := currentState

	val := reflect.ValueOf(&newState).Elem()
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("flodk_extraction")

		if tag == "" {
			continue
		}

		if extractedVal, ok := extractedValue[tag]; ok {
			fieldVal := val.Field(i)
			if fieldVal.CanSet() {
				extractedReflectVal := reflect.ValueOf(extractedVal)
				if extractedReflectVal.Type().AssignableTo(fieldVal.Type()) {
					fieldVal.Set(extractedReflectVal)
				}
			}
		}
	}

	return newState
}

// JSONKeySet is a StateUpdateFunc which is just decodes the extracted values as JSON into the
// passed state. Just be warned that sometimes some undesired struct fields can be overwritten.
// because of any unexpected response from the LLMs.
func JSONKeySet[T any](currentState T, extractedValues map[string]any) T {
	bs, err := json.Marshal(extractedValues)
	if err != nil {
		return currentState
	}

	newState := currentState

	err = json.Unmarshal(bs, &newState)
	if err != nil {
		return newState
	}

	return newState
}
