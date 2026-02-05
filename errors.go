package flodk

import "fmt"

type InterruptID struct {
	NodeID string `json:"node_id"`
	ID     string `json:"id"`
}

func (i InterruptID) String() string {
	return i.NodeID + ":" + i.ID
}

type RequirementTypes string

const (
	Enum                  RequirementTypes = "enum"
	Custom                RequirementTypes = "custom"
	CustomWithSuggestions RequirementTypes = "custom_with_suggestions"
)

type Requirement struct {
	Type        RequirementTypes `json:"type"`
	Suggestions []string         `json:"suggestions"`
}

type Requirements map[string]Requirement

func (r Requirements) Validate(values map[string]string) error {
	for k := range r {
		data, ok := values[k]
		if !ok {
			return fmt.Errorf("key %s not found in interrupt response", k)
		}

		if data == "" {
			return fmt.Errorf("%s cannot be empty", k)
		}
	}

	return nil
}

// HITLInterrupt is used to return a invoke a human in the loop
// routine as a part of the flow.
type HITLInterrupt struct {
	Reason          string       `json:"reason"`
	Message         string       `json:"message"`
	ValidationError error        `json:"validation_error"`
	Requirements    Requirements `json:"requirements"`
	InterruptID     InterruptID  `json:"interrupt_id"`
}

// Error implements the error interface for the task interrupt.
func (it HITLInterrupt) Error() string {
	return fmt.Sprintf("flow interrupted: %s", it.Reason)
}

// ConditionalInterrupt is used to direct the execution of a flow
// using a alias value. This value will then be used to choose the
// next edge of the graph.
type ConitionalInterrupt struct {
	Value string
}

// Error implements the error interface for the conditional interrupt.
func (ci ConitionalInterrupt) Error() string {
	return fmt.Sprintf("conditional interrupt: directing to %s", ci.Value)
}
