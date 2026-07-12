package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/kodalabss/silo-sdk-go/internal/compiler"
	"github.com/kodalabss/silo-sdk-go/internal/factory"
)

const BrainURL = "https://brain.silobase.com"

func main() {
	if len(os.Args) < 2 {
		runInteractiveMenu()
		return
	}

	command := os.Args[1]
	switch command {
	case "build":
		handleBuild()
	case "generate":
		handleGenerate()
	case "explain":
		handleExplain()
	case "trace":
		handleTrace()
	case "doctor":
		handleDoctor()
	default:
		printUsage()
	}
}

func runInteractiveMenu() {
	fmt.Println("\n=== SILOBASE COMMAND CENTER ===")
	fmt.Println("1. [Schema]    Build & Push")
	fmt.Println("2. [Factory]   Generate SDK (Twin-Link)")
	fmt.Println("3. [Explain]   Diagnose Request ID")
	fmt.Println("4. [Doctor]    Workspace Health Scan")
	fmt.Println("5. [Radar]     Deep Health Scan")
	fmt.Println("q. Exit")
	fmt.Print("\nChoice: ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1":
		handleBuildInteractive()
	case "2":
		handleGenerateInteractive()
	case "3":
		var id string
		fmt.Print("Request ID: ")
		fmt.Scanln(&id)
		callDebugAPI("explain", id)
	case "4":
		callDebugAPI("doctor", "")
	case "q":
		os.Exit(0)
	default:
		fmt.Println("Invalid choice. bruh.")
	}
}

func callDebugAPI(command, id string) {
	url := fmt.Sprintf("%s/debug/%s?id=%s", BrainURL, command, id)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error contacting Brain: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&res)

	pretty, _ := json.MarshalIndent(res, "", "  ")
	fmt.Printf("\n--- BRAIN DIAGNOSIS ---\n%s\n", string(pretty))
}

func handleBuildInteractive() {
	fmt.Println("\n[Schema] Building Sovereign Reality...")
	rm, err := compiler.Build("./schema")
	if err != nil {
		fmt.Printf("Build failed: %v\n", err)
		return
	}
	fmt.Printf("SUCCESS: Compiled %d trajectories. Ready for push. bruh.\n", len(rm.Dimensions))
}

func handleBuild() {
	rm, err := compiler.Build("./schema")
	if err != nil {
		fmt.Printf("Build failed: %v\n", err)
		return
	}
	fmt.Printf("SUCCESS: Built manifest v%d with %d signatures.\n", rm.Version, len(rm.Signatures))
}

func handleGenerateInteractive() {
	var ws, lang string
	fmt.Print("\n[Factory] Reality Generation Start")
	fmt.Print("\nWorkspace ID: ")
	fmt.Scanln(&ws)
	fmt.Print("Client Language (kotlin/swift/ts/dart): ")
	fmt.Scanln(&lang)

	cfg := factory.Config{
		WorkspaceID: ws,
		BaseURL:     BrainURL,
		Language:    lang,
	}

	err := factory.Generate(cfg, "./out")
	if err != nil {
		fmt.Printf("\nGeneration failed: %v\n", err)
	} else {
		fmt.Println("\nSUCCESS: Reality Twin generated in ./out")
	}
}

func handleGenerate() {
	// CLI Flag implementation ...
}

func handleExplain() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: silo explain <request_id>")
		return
	}
	callDebugAPI("explain", os.Args[2])
}

func handleTrace() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: silo trace <request_id>")
		return
	}
	callDebugAPI("trace", os.Args[2])
}

func handleDoctor() {
	callDebugAPI("doctor", "")
}

func printUsage() {
	fmt.Println("Silobase CLI - The Reality Factory")
	fmt.Println("Usage: silo <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  build       Build local schema")
	fmt.Println("  generate    Generate Twin-Link SDK")
	fmt.Println("  explain     Diagnose a Request ID")
	fmt.Println("  trace       Show flight recorder for ID")
	fmt.Println("  doctor      Check workspace health")
}
