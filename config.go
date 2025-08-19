package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"
)

type Config struct {
	query                 string
	queries               []string
	outputJSON            bool
	verbose               bool
	stream                bool
	workers               int
	timeout               time.Duration
	includeSummary        bool
	includeSummaryExplicit bool
}

type SearchResult struct {
	Query     string        `json:"query"`
	Response  string        `json:"response"`
	Summary   string        `json:"summary,omitempty"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

type MultiSearchResult struct {
	Results   []SearchResult `json:"results"`
	TotalTime time.Duration  `json:"total_time"`
	Success   bool           `json:"success"`
	Error     string         `json:"error,omitempty"`
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.query, "query", "", "Single search query")
	flag.BoolVar(&config.outputJSON, "json", false, "Output in JSON format")
	flag.BoolVar(&config.verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&config.verbose, "v", false, "Enable verbose logging (shorthand)")
	flag.BoolVar(&config.stream, "stream", false, "Stream results as they complete")
	flag.IntVar(&config.workers, "workers", 3, "Max concurrent queries (1-5)")
	flag.DurationVar(&config.timeout, "timeout", 180*time.Second, "Total operation timeout")

	// Custom flag for include-summary to track explicit setting
	flag.Func("include-summary", "Include AI-generated summaries (default: off for single query, on for multi-query)", func(value string) error {
		config.includeSummaryExplicit = true
		if value == "true" || value == "" {
			config.includeSummary = true
		} else {
			config.includeSummary = false
		}
		return nil
	})

	// Custom flag for multiple queries
	flag.Func("q", "Search query (can be repeated)", func(value string) error {
		config.queries = append(config.queries, value)
		return nil
	})

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [query]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A CLI search engine powered by Gemini AI\n\n")
		fmt.Fprintf(os.Stderr, "Note: When using positional arguments, flags must come BEFORE the query.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s \"What is Go programming?\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -include-summary \"What is Go programming?\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -q \"Go\" -q \"Python\" -q \"Rust\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -q \"Go\" -q \"Python\" -include-summary=false\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -stream \"What is Go programming?\"\n", os.Args[0])
	}

	flag.Parse()

	// Handle positional argument
	if config.query == "" && len(config.queries) == 0 && len(flag.Args()) > 0 {
		config.query = flag.Args()[0]
	}

	// Set smart defaults for includeSummary if not explicitly set by user
	if !config.includeSummaryExplicit {
		totalQueries := 0
		if config.query != "" {
			totalQueries++
		}
		totalQueries += len(config.queries)
		
		if totalQueries == 1 {
			// Single query (either positional or single -q): summary OFF by default
			config.includeSummary = false
		} else if totalQueries > 1 {
			// Multiple queries: summary ON by default
			config.includeSummary = true
		}
	}

	return config
}

func validateConfig(config *Config) error {
	hasQuery := config.query != ""
	hasQueries := len(config.queries) > 0

	if !hasQuery && !hasQueries {
		return fmt.Errorf("search query is required (use -query, -q, or positional argument)")
	}
	if hasQuery && hasQueries {
		return fmt.Errorf("cannot use both -query and -q flags simultaneously")
	}
	if config.workers < 1 || config.workers > 5 {
		return fmt.Errorf("workers must be between 1 and 5")
	}
	if config.stream && hasQueries {
		return fmt.Errorf("streaming mode is not supported for multiple queries (use single query only)")
	}
	return nil
}

func setupLogger(verbose bool) {
	level := slog.LevelError
	if verbose {
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}

func handleError(err error, context string) {
	slog.Error(context, "error", err)
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n", context, err)
	os.Exit(1)
}