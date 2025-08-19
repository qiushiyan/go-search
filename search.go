package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"google.golang.org/genai"
)

//go:embed prompts/system.txt
var systemInstructionText string

//go:embed prompts/summary.txt
var summaryInstructionText string

var tools = []*genai.Tool{
	{GoogleSearch: &genai.GoogleSearch{}},
	{URLContext: &genai.URLContext{}},
}

var thinkingBudget int32 = 512
var model = "gemini-2.5-flash"

func getSystemInstruction() *genai.Content {
	return &genai.Content{
		Parts: []*genai.Part{{
			Text: systemInstructionText,
		}},
	}
}

func getSummaryInstruction() *genai.Content {
	return &genai.Content{
		Parts: []*genai.Part{{
			Text: summaryInstructionText,
		}},
	}
}

func initializeClient(ctx context.Context) (*genai.Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return client, nil
}

func performSingleSearch(ctx context.Context, query string, client *genai.Client) (*SearchResult, error) {
	startTime := time.Now()
	result := &SearchResult{
		Query:     query,
		Timestamp: startTime,
	}

	isoDateString := time.Now().Format(time.DateOnly)
	parts := []*genai.Part{
		{Text: fmt.Sprintf(`
<query>
%s
</query>

Time Context: today is %s

`, query, isoDateString)},
	}
	content := []*genai.Content{{
		Role:  "user",
		Parts: parts,
	}}

	slog.Info("Performing search", "query", query)

	// Simple retry logic - try twice with 3 second delay
	var response *genai.GenerateContentResponse
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		response, err = client.Models.GenerateContent(ctx, model, content, &genai.GenerateContentConfig{
			SystemInstruction: getSystemInstruction(),
			Tools:             tools,
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: &thinkingBudget,
			},
		})

		if err == nil && response.Text() != "" {
			break
		}

		if attempt == 0 {
			slog.Info("Retrying search request", "query", query, "attempt", attempt+2)
			time.Sleep(3 * time.Second)
		}
	}

	result.Duration = time.Since(startTime)

	if err != nil {
		result.Error = "Search failed"
		result.Success = false
		return result, fmt.Errorf("failed to generate content after retries: %w", err)
	}

	if response.Text() == "" {
		result.Error = "Empty response"
		result.Success = false
		return result, fmt.Errorf("received empty response after retries")
	}

	result.Response = response.Text()
	result.Success = true
	return result, nil
}

func performSingleSearchStream(ctx context.Context, query string, client *genai.Client) (*SearchResult, error) {
	startTime := time.Now()
	result := &SearchResult{
		Query:     query,
		Timestamp: startTime,
	}

	isoDateString := time.Now().Format(time.DateOnly)
	parts := []*genai.Part{
		{Text: fmt.Sprintf(`
<query>
%s
</query>

Time Context: today is %s

`, query, isoDateString)},
	}
	content := []*genai.Content{{
		Role:  "user",
		Parts: parts,
	}}

	slog.Info("Performing search", "query", query)

	fmt.Printf("\n=== %s ===\n", query)

	var responseText string
	var lastErr error

	// Simple retry logic for streaming - try twice with 3 second delay
	for attempt := 0; attempt < 2; attempt++ {
		responseText = ""

		iterator := client.Models.GenerateContentStream(ctx, model, content, &genai.GenerateContentConfig{
			SystemInstruction: getSystemInstruction(),
			Tools:             tools,
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: &thinkingBudget,
			},
		})

		streamSuccess := true
		for response, err := range iterator {
			if err != nil {
				lastErr = err
				streamSuccess = false
				break
			}

			if len(response.Candidates) > 0 {
				chunk := response.Text()
				fmt.Print(chunk)
				responseText += chunk
			}
		}

		if streamSuccess && responseText != "" {
			break
		}

		if attempt == 0 {
			slog.Info("Retrying stream search request", "query", query, "attempt", attempt+2)
			fmt.Printf("\n[Retrying...]\n")
			time.Sleep(3 * time.Second)
		}
	}

	fmt.Printf("\n%s\n", "─────────────────────────────────────────────────────────────────────────────")

	result.Duration = time.Since(startTime)

	if lastErr != nil && responseText == "" {
		result.Error = "Stream search failed"
		result.Success = false
		return result, fmt.Errorf("failed to stream content after retries: %w", lastErr)
	}

	if responseText == "" {
		result.Error = "Empty stream response"
		result.Success = false
		return result, fmt.Errorf("received empty stream response after retries")
	}

	result.Response = responseText
	result.Success = true
	return result, nil
}

func generateSummary(ctx context.Context, query, response string, client *genai.Client) (string, error) {
	parts := []*genai.Part{
		{Text: fmt.Sprintf("Query: %s\n\nSearch Results:\n%s", query, response)},
	}
	content := []*genai.Content{{
		Role:  "user",
		Parts: parts,
	}}

	// Simple retry logic for summary
	var result *genai.GenerateContentResponse
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		result, err = client.Models.GenerateContent(ctx, model, content, &genai.GenerateContentConfig{
			SystemInstruction: getSummaryInstruction(),
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: &thinkingBudget,
			},
		})

		if err == nil && result.Text() != "" {
			break
		}

		if attempt == 0 {
			time.Sleep(3 * time.Second)
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate summary after retries: %w", err)
	}

	if result.Text() == "" {
		return "", fmt.Errorf("received empty summary after retries")
	}

	return result.Text(), nil
}

func processMultipleQueries(ctx context.Context, queries []string, config *Config, client *genai.Client) (*MultiSearchResult, error) {
	startTime := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, config.timeout)
	defer cancel()

	results := make([]SearchResult, len(queries))
	var wg sync.WaitGroup
	sem := make(chan struct{}, config.workers) // Simple semaphore for concurrency control

	for i, query := range queries {
		wg.Add(1)
		go func(index int, q string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			result := processQuery(ctx, q, client, config.includeSummary)
			results[index] = result

			if config.verbose {
				slog.Info("Query completed", "query", result.Query, "success", result.Success, "duration", result.Duration)
			}
		}(i, query)
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	// Calculate success count
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	if config.verbose {
		slog.Info("Query execution completed",
			"total_queries", len(queries),
			"successful", successCount,
			"total_duration", totalTime.Round(time.Millisecond))
	}

	multiResult := &MultiSearchResult{
		Results:   results,
		TotalTime: totalTime,
		Success:   successCount == len(queries),
	}

	if !multiResult.Success {
		multiResult.Error = fmt.Sprintf("Completed %d/%d queries successfully", successCount, len(queries))
	}

	return multiResult, nil
}

func processQuery(ctx context.Context, query string, client *genai.Client, includeSummary bool) SearchResult {
	startTime := time.Now()

	result := SearchResult{
		Query:     query,
		Timestamp: startTime,
	}

	// Perform regular search (no streaming for multi-query)
	searchResult, err := performSingleSearch(ctx, query, client)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}

	result.Response = searchResult.Response
	result.Success = searchResult.Success
	result.Duration = searchResult.Duration

	// Generate summary if requested
	if result.Success && includeSummary {
		summary, err := generateSummary(ctx, query, result.Response, client)
		if err != nil {
			result.Summary = "Summary generation failed"
		} else {
			result.Summary = summary
		}
	}

	return result
}

func (r *SearchResult) Output(outputJSON bool) error {
	if outputJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(r)
	}

	if !r.Success {
		fmt.Fprintf(os.Stderr, "Search failed: %s\n", r.Error)
		return fmt.Errorf("search failed")
	}

	// Show summary first if available
	if r.Summary != "" {
		fmt.Printf("## SUMMARY\n%s\n\n", r.Summary)
		fmt.Printf("## DETAILED RESPONSE\n")
	}
	
	fmt.Println(r.Response)
	return nil
}

func (m *MultiSearchResult) Output(outputJSON bool, isStream bool, includeSummary bool) error {
	if outputJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(m)
	}

	if isStream {
		// In stream mode, results already shown, just show completion
		successful := 0
		for _, result := range m.Results {
			if result.Success {
				successful++
			}
		}
		fmt.Printf("\n🏁 COMPLETED: %d/%d queries\n", successful, len(m.Results))
		return nil
	}

	// Calculate success/failure counts
	successful := 0
	failed := 0
	for _, result := range m.Results {
		if result.Success {
			successful++
		} else {
			failed++
		}
	}

	if includeSummary {
		// Combined overview and summaries section
		fmt.Printf("## SEARCH RESULTS\n")
		fmt.Printf("%d/%d queries completed successfully, here is a summary for each query:\n\n", successful, len(m.Results))

		for _, result := range m.Results {
			if result.Success {
				summary := result.Summary
				if summary == "" {
					summary = "No summary available"
				}
				fmt.Printf("✓ %s: %s\n", result.Query, summary)
			} else {
				fmt.Printf("✗ %s: %s\n", result.Query, result.Error)
			}
		}
		fmt.Printf("\n")

		// Detailed responses section (without durations)
		fmt.Printf("## DETAILED RESPONSES\n\n")
	}
	for _, result := range m.Results {
		if len(m.Results) > 1 {
			fmt.Printf("=== %s ===\n", result.Query)
		}
		if result.Success {
			fmt.Printf("%s\n", result.Response)
		} else {
			fmt.Printf("Status: FAILED - %s\n", result.Error)
		}
		fmt.Printf("\n")
	}

	return nil
}
