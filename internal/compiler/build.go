package compiler

import (
	"fmt"
	"os"
	"path/filepath"
)

func Build(dir string) (*ResolvedManifest, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	combined := &Manifest{Version: 1}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".silo" {
			continue
		}

		path := filepath.Join(dir, file.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		parser := NewParser(string(content))
		m, err := parser.Parse()
		if err != nil {
			return nil, fmt.Errorf("file %s: %v", file.Name(), err)
		}

		if m.Version > combined.Version {
			combined.Version = m.Version
		}

		combined.Entries = append(combined.Entries, m.Entries...)
	}

	if err := Validate(combined); err != nil {
		return nil, err
	}

	return Resolve(combined), nil
}
