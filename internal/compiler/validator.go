package compiler

import (
	"fmt"
	"strings"
)

type IntentRule struct {
	RequiredParams []string
	Conflicts      []string
}

var IntentRegistry = map[string]IntentRule{
	"rank": {
		RequiredParams: []string{"width"},
		Conflicts:      []string{"sub"},
	},
	"match": {
		RequiredParams: []string{},
		Conflicts:      []string{},
	},
	"range": {
		RequiredParams: []string{"width"},
		Conflicts:      []string{"sub"},
	},
	"stamp": {
		RequiredParams: []string{"interval"},
		Conflicts:      []string{},
	},
	"unique": {
		RequiredParams: []string{},
		Conflicts:      []string{},
	},
	"count": {
		RequiredParams: []string{},
		Conflicts:      []string{},
	},
	"sub": {
		RequiredParams: []string{},
		Conflicts:      []string{"rank", "range"},
	},
}

func Validate(m *Manifest) error {
	for _, entry := range m.Entries {
		if err := validateEntry(entry); err != nil {
			return err
		}
	}
	return nil
}

func validateEntry(e Entry) error {
	// 1. Case Law (Lowercase identifiers)
	if e.Name != strings.ToLower(e.Name) {
		return fmt.Errorf("line %d: identifier '%s' must be lowercase", e.Line, e.Name)
	}

	// 2. Intent Validation
	for _, intent := range e.Intents {
		rule, ok := IntentRegistry[intent.Name]
		if !ok {
			return fmt.Errorf("line %d: unknown intent '@%s'", intent.Line, intent.Name)
		}

		// Required Params
		for _, req := range rule.RequiredParams {
			if _, exists := intent.Params[req]; !exists {
				return fmt.Errorf("line %d: intent '@%s' missing required parameter '%s'", intent.Line, intent.Name, req)
			}
		}

		// Conflict Law
		for _, other := range e.Intents {
			if intent.Name == other.Name {
				continue
			}
			for _, conflict := range rule.Conflicts {
				if other.Name == conflict {
					return fmt.Errorf("line %d: intent conflict: '@%s' cannot be combined with '@%s'", intent.Line, intent.Name, other.Name)
				}
			}
		}
	}

	// Recursive check
	for _, child := range e.Children {
		if err := validateEntry(child); err != nil {
			return err
		}
	}

	return nil
}
