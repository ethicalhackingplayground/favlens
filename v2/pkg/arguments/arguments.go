package args

import (
	"flag"
	"fmt"

	"github.com/fatih/color"
)

// Arguments struct to hold command line arguments
type Arguments struct {
	BaseURL        string
	OllamaHost     string
	FilePath       string
	Model          string
	Workers        int
	Debug          bool
	Verbose        bool
	Silent         bool
	Output         string
	TimeoutSeconds int
	DelayMs        int
}

func NewArguments() *Arguments {
	// CLI flags
	baseURL := flag.String("base", "", "Base favicon URL to compare against (required)")
	ollamaHost := flag.String("ollama-host", "http://localhost:11434", "Ollama host (default: http://localhost:11434)")
	filePath := flag.String("file", "", "Path to file containing URLs to check (required)")
	model := flag.String("model", "gemma3:4b", "Ollama model to use (default: gemma3:4b)")
	workers := flag.Int("workers", 5, "Number of concurrent workers (default: 5)")
	debug := flag.Bool("debug", false, "Enable debug logging (shows everything)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging (shows info without errors)")
	silent := flag.Bool("silent", false, "Silent mode (only shows matched URLs)")
	output := flag.String("o", "", "Output file to save matched URLs (optional)")
	timeoutSeconds := flag.Int("timeout", 30, "HTTP timeout in seconds (default: 30)")
	delayMs := flag.Int("delay", 0, "Delay between requests in milliseconds (default: 0)")

	// Parse flags before returning values
	flag.Parse()

	return &Arguments{
		BaseURL:        *baseURL,
		OllamaHost:     *ollamaHost,
		FilePath:       *filePath,
		Model:          *model,
		Workers:        *workers,
		Debug:          *debug,
		Verbose:        *verbose,
		Silent:         *silent,
		Output:         *output,
		TimeoutSeconds: *timeoutSeconds,
		DelayMs:        *delayMs,
	}
}

func (a *Arguments) IsValid() bool {
	return a.BaseURL != "" && a.FilePath != "" && a.Model != ""
}

func (a *Arguments) Parse() (Arguments, error) {
	if !a.IsValid() {
		return Arguments{}, fmt.Errorf("invalid arguments")
	}
	// Already parsed in NewArguments(); keep for backward compatibility
	return *a, nil
}

func PrintBanner() {
	// Print Ascii Art in white bold
	banner := `                            
 _____         __                
|   __|___ _ _|  |   ___ ___ ___ 
|   __| .'| | |  |__| -_|   |_ -|
|__|  |__,|\_/|_____|___|_|_|___|
                                    
    `
	color.New(color.FgWhite, color.Bold).Fprintln(color.Output, banner)
	// Tagline in italic cyan
	color.New(color.Italic, color.FgCyan).Fprintln(color.Output, "Compare favicons against a base URL using Ollama models")
}
