package llm

// StateContract defines a contract for the generic type passed as a state
// to the LLM nodes so that the prompt can be passed to the LLM for any natural
// language analysis.
type StateContract interface {
	Prompt() string
}
