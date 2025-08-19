package main

import (
	"context"
	"os"
)

func main() {
	config := parseFlags()
	
	if err := validateConfig(config); err != nil {
		handleError(err, "Configuration validation failed")
	}
	
	setupLogger(config.verbose)
	
	ctx := context.Background()
	client, err := initializeClient(ctx)
	if err != nil {
		handleError(err, "Failed to initialize client")
	}
	
	// Handle single query
	if config.query != "" {
		var result *SearchResult
		var err error
		
		if config.stream {
			result, err = performSingleSearchStream(ctx, config.query, client)
		} else {
			result, err = performSingleSearch(ctx, config.query, client)
		}
		
		if err != nil {
			handleError(err, "Search failed")
		}
		
		// In stream mode, output is already shown, just exit
		if config.stream {
			if !result.Success {
				os.Exit(1)
			}
			return
		}
		
		// Generate summary for single query if requested
		if config.includeSummary && result.Success {
			summary, err := generateSummary(ctx, result.Query, result.Response, client)
			if err != nil {
				result.Summary = "Summary generation failed"
			} else {
				result.Summary = summary
			}
		}
		
		if err := result.Output(config.outputJSON); err != nil {
			os.Exit(1)
		}
		return
	}
	
	// Handle multiple queries
	if len(config.queries) > 0 {
		multiResult, err := processMultipleQueries(ctx, config.queries, config, client)
		if err != nil {
			handleError(err, "Multi-query search failed")
		}
		
		if err := multiResult.Output(config.outputJSON, config.stream, config.includeSummary); err != nil {
			os.Exit(1)
		}
		
		if !multiResult.Success {
			os.Exit(1)
		}
	}
}