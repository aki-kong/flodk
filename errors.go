package flodk

import (
	"errors"
	"fmt"
	"strings"
)

type ErrRequirmentKeyNotFound string

func (k ErrRequirmentKeyNotFound) Error() string {
	return "requirement key '" + string(k) + "' not found"
}

func RequirementKeyNotFound(key string) ErrRequirmentKeyNotFound {
	return ErrRequirmentKeyNotFound(key)
}

type ErrRequirementInvalidValue struct {
	Key         string
	Value       string
	Suggestions string
}

func RequirementInvalid(key string, value string, suggestions []string) ErrRequirementInvalidValue {
	return ErrRequirementInvalidValue{
		Key:         key,
		Value:       value,
		Suggestions: strings.Join(suggestions, ", "),
	}
}

func (iv ErrRequirementInvalidValue) Error() string {
	return fmt.Sprintf("invalid value for %s: %s, need one of [%s]", iv.Key, iv.Value, iv.Suggestions)
}

var ErrFlowExecutionEnded = errors.New("flow execution ended")
