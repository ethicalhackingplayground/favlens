package main

import (
	"fmt"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	"os"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/webp" // Register WebP format

	args "github.com/ethicalhackingplayground/favlens/pkg/arguments"
	"github.com/ethicalhackingplayground/favlens/pkg/ollama"
	"github.com/ethicalhackingplayground/favlens/pkg/types"
	"github.com/fatih/color"
	_ "github.com/mat/besticon/ico" // Register ICO format
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/levels"
)

type Job struct {
	URL string
}

// Worker function that processes jobs from the job channel
func worker(id int, jobs <-chan Job, results chan<- types.Result, baseIcon string, ollamaClient *ollama.Client, args *args.Arguments, wg *sync.WaitGroup) {
	defer wg.Done()

	if args.Debug {
		gologger.Debug().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Worker %d started", id))
	}

	processedCount := 0
	for job := range jobs {
		processedCount++
		if args.Debug {
			gologger.Debug().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Worker %d processing job %d: %s", id, processedCount, job.URL))
		}

		// Optional delay between requests
		if args.DelayMs > 0 {
			time.Sleep(time.Duration(args.DelayMs) * time.Millisecond)
		}

		targetIcon, err := ollamaClient.DownloadImageAsBase64(job.URL, args.Debug)
		if err != nil {
			if args.Debug {
				gologger.Debug().Msg(color.New(color.Italic, color.FgRed).Sprintf("Worker %d failed to download %s: %v", id, job.URL, err))
			}
			results <- types.Result{URL: job.URL, Match: false, Err: err}
			continue
		}

		match, err := ollamaClient.CompareFaviconsChatAPI(baseIcon, targetIcon, args.Debug)
		if args.Debug {
			gologger.Debug().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Worker %d completed comparison for %s: match=%v, err=%v", id, job.URL, match, err))
		}
		results <- types.Result{URL: job.URL, Match: match, Err: err}
	}

	if args.Debug {
		gologger.Debug().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Worker %d finished, processed %d jobs", id, processedCount))
	}
}

func main() {
	args.PrintBanner()

	args := args.NewArguments()

	if !args.IsValid() {
		fmt.Println(color.New(color.FgYellow, color.Italic).Sprint("Usage: go run main.go --base <base_favicon_url> --file <url_list_file> [--model <model_name>] [--workers <num>] [--timeout <seconds>] [--delay <ms>] [--debug|--verbose|--silent] [-o <output_file>]"))
		os.Exit(1)
	}

	// Configure logger based on flags
	if args.Silent {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelSilent)
	} else if args.Debug {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelDebug)
		gologger.Info().Msg(color.New(color.Italic, color.FgMagenta).Sprint("Debug logging enabled"))
	} else if args.Verbose {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelVerbose)
	} else {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelInfo)
	}

	if !args.Silent {
		gologger.Info().Msg(color.New(color.Bold, color.FgGreen).Sprint("Starting favicon comparison tool"))
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Base URL: %s", args.BaseURL))
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Model: %s", args.Model))
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Workers: %d", args.Workers))
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Timeout: %ds", args.TimeoutSeconds))
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Delay: %dms", args.DelayMs))
		if args.Output != "" {
			gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Output file: %s", args.Output))
		}
	}

	// Create a new Ollama client
	ollamaClient := ollama.NewClient(args.OllamaHost, args.Model, time.Duration(args.TimeoutSeconds)*time.Second)

	// Check if the specified model exists before proceeding
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Italic, color.FgYellow).Sprintf("Validating model '%s' availability...", args.Model))
	}
	if err := ollamaClient.CheckModelExists(args.Debug); err != nil {
		if args.Silent {
			os.Exit(1)
		}
		gologger.Fatal().Msg(color.New(color.Bold, color.FgRed).Sprintf("Model validation failed: %v", err))
	}
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Bold, color.FgGreen).Sprintf("Model '%s' is available", args.Model))
	}

	// Download base favicon
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Italic, color.FgYellow).Sprint("Downloading base favicon..."))
	}

	baseIcon, err := ollamaClient.DownloadImageAsBase64(args.BaseURL, args.Debug)
	if err != nil {
		if args.Silent {
			os.Exit(1)
		}
		gologger.Fatal().Msg(color.New(color.Bold, color.FgRed).Sprintf("Failed to download base favicon: %v", err))
	}
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Bold, color.FgGreen).Sprint("Base favicon downloaded successfully"))
	}

	// Read file with URLs
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Reading URLs from file: %s", args.FilePath))
	}
	content, err := os.ReadFile(args.FilePath)
	if err != nil {
		if args.Silent {
			os.Exit(1)
		}
		gologger.Fatal().Msg(color.New(color.Bold, color.FgRed).Sprintf("Failed to read file: %v", err))
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Found %d URLs to process", len(lines)))
	}

	// Create channels
	jobs := make(chan Job, len(lines))
	results := make(chan types.Result, len(lines))

	// Start worker pool
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Bold, color.FgGreen).Sprintf("Starting %d workers...", args.Workers))
	}
	var wg sync.WaitGroup
	for i := 0; i < args.Workers; i++ {
		wg.Add(1)
		go worker(i, jobs, results, baseIcon, ollamaClient, args, &wg)
	}

	// Send jobs
	jobCount := 0
	for _, url := range lines {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}

		// Append /favicon.ico if the URL doesn't have an image extension or favicon.ico
		if !strings.HasSuffix(url, ".ico") && !strings.HasSuffix(url, ".png") &&
			!strings.HasSuffix(url, ".jpg") && !strings.HasSuffix(url, ".jpeg") &&
			!strings.HasSuffix(url, ".gif") && !strings.HasSuffix(url, ".svg") &&
			!strings.Contains(url, "favicon") {
			if strings.HasSuffix(url, "/") {
				url = url + "favicon.ico"
			} else {
				url = url + "/favicon.ico"
			}
			if args.Debug {
				gologger.Debug().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Appended /favicon.ico to URL: %s", url))
			}
		}

		jobs <- Job{URL: url}
		jobCount++
	}
	close(jobs)
	if !args.Silent {
		gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Dispatched %d jobs to workers", jobCount))
	}

	// Wait for all workers to finish and close results channel
	go func() {
		wg.Wait()
		close(results)
		if !args.Silent {
			gologger.Info().Msg(color.New(color.Bold, color.FgGreen).Sprint("All workers finished"))
		}
	}()

	// Prepare output file if specified
	var outFile *os.File
	if args.Output != "" {
		outFile, err = os.Create(args.Output)
		if err != nil {
			if args.Silent {
				os.Exit(1)
			}
			gologger.Fatal().Msg(color.New(color.Bold, color.FgRed).Sprintf("Failed to create output file: %v", err))
		}
		defer outFile.Close()
		if !args.Silent {
			gologger.Info().Msg(color.New(color.Italic, color.FgCyan).Sprintf("Created output file: %s", args.Output))
		}
	}

	// Collect and print results
	matchCount := 0
	errorCount := 0
	for result := range results {
		if result.Err != nil {
			errorCount++
			// Only show errors in debug mode
			if args.Debug {
				gologger.Debug().Msg(color.New(color.Italic, color.FgRed).Sprintf("Error processing %s: %v", result.URL, result.Err))
			}
			continue
		}
		if result.Match {
			matchCount++
			fmt.Println(result.URL)

			// Write to output file if specified
			if outFile != nil {
				if _, err := fmt.Fprintln(outFile, result.URL); err != nil {
					if args.Debug {
						gologger.Debug().Msg(color.New(color.Italic, color.FgRed).Sprintf("Failed to write to output file: %v", err))
					}
				}
			}
		}
	}

	if !args.Silent {
		gologger.Info().Msg(color.New(color.Bold, color.FgGreen).Sprintf("Processing complete. Matches: %d, Errors: %d, Total: %d", matchCount, errorCount, jobCount))
		if args.Output != "" {
			gologger.Info().Msg(color.New(color.Bold, color.FgGreen).Sprintf("Matched URLs saved to: %s", args.Output))
		}
	}
}
