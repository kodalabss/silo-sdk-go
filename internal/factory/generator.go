package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
)

type Config struct {
	WorkspaceID string
	BaseURL     string
	AuthURL     string
	KeyVar      string
	Language    string
	Backend     string
}

func Generate(cfg Config, outDir string) error {
	_, filename, _, _ := runtime.Caller(0)
	tmplDir := filepath.Join(filepath.Dir(filename), "templates")

	// 1. Verify Role/Language Alignment (Sovereign Guard)
	if !isFrontend(cfg.Language) {
		return fmt.Errorf("SOVEREIGNTY_VIOLATION: %s is a backend language. The Pulse MUST live on the user's frontend.", cfg.Language)
	}

	// 2. Generate Client (Pulse Engine)
	clientTmpl := filepath.Join(tmplDir, "client_"+cfg.Language+".tmpl")
	if err := render(clientTmpl, cfg, filepath.Join(outDir, "SiloPulse."+cfg.Language)); err != nil {
		return err
	}

	return nil
}

func isFrontend(lang string) bool {
	frontend := map[string]bool{
		"kotlin":     true,
		"swift":      true,
		"typescript": true,
		"ts":         true,
		"dart":       true,
	}
	return frontend[lang]
}

func render(tmplPath string, data interface{}, outPath string) error {
	t, err := template.ParseFiles(tmplPath)
	if err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}
