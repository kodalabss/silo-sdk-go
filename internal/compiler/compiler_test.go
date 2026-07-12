package compiler

import (
	"fmt"
	"testing"
)

func TestCompiler(t *testing.T) {
	input := `
version 1

user {
    [id] {
        score {
            @rank(width: 100)
            @match
        }
        username {
            @match
            @unique
        }
    }
}
`
	parser := NewParser(input)
	manifest, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if err := Validate(manifest); err != nil {
		t.Fatalf("Validation error: %v", err)
	}

	resolved := Resolve(manifest)
	fmt.Printf("Resolved Manifest Version: %d\n", resolved.Version)
	fmt.Printf("Signatures: %d\n", len(resolved.Signatures))
	for path, sig := range resolved.Signatures {
		fmt.Printf("  %s -> %d\n", path, sig)
	}
	fmt.Printf("Dimensions: %d\n", len(resolved.Dimensions))
}
