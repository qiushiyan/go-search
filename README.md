# Search CLI

A CLI search engine powered by Gemini AI with intelligent summarization and concurrent query processing.

## Prerequites

You should have environmentals set up for either **Gemini Developer AI** or **Vertex AI**.

**Gemini Developer API:** Set `GOOGLE_API_KEY` as shown below:

```bash
export GOOGLE_API_KEY='your-api-key'
```

**Gemini API on Vertex AI:** Set `GOOGLE_GENAI_USE_VERTEXAI`,
`GOOGLE_CLOUD_PROJECT` and `GOOGLE_CLOUD_LOCATION`, as shown below:

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT='your-project-id'
export GOOGLE_CLOUD_LOCATION='us-central1'
```


See https://github.com/googleapis/go-genai for details.



## Features

- **Single & Multi-Query Search**: Search one topic or compare multiple topics concurrently
- **Smart Summaries**: AI-generated 1-3 sentence summaries with actionable insights
- **Summary-First Output**: Quick overview section before detailed responses
- **Stream Mode**: Real-time results for single queries (multi-query not supported)
- **Multiple Input Methods**: Positional arguments, single flags, or repeatable flags
- **JSON Output**: Structured output for integration and automation
- **Robust Error Handling**: Automatic retry logic and clear failure reporting

## Usage

### Single Query
```bash
./search "your search query"
./search -query "your search query"

# With flags (flags must come BEFORE positional query)
./search -v "your search query"
./search -json "your search query"
```

### Multiple Queries
```bash
# Standard mode with summaries (streaming not supported)
./search -q "query1" -q "query2" -q "query3"
```

### Streaming Mode
```bash
# Single query streaming only
./search -stream "your search query"
```

## Output Formats

### Single Query Output
```
[Direct response content]
```

### Multi-Query Output (Standard Mode)
```
## SEARCH OVERVIEW
Completed: 2/2 queries

## SUMMARIES
✓ Go: Statically typed language optimized for concurrent systems and modern development practices
✓ Python: Interpreted language emphasizing readability, rapid development, and extensive ecosystem

## DETAILED RESPONSES

=== Go ===

[Full detailed response about Go...]

=== Python ===

[Full detailed response about Python...]
```


### JSON Output
```bash
./search -q "Go" -q "Python" -json
```
Returns structured JSON with metadata including success status and timestamps.

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `-query` | Single search query | - |
| `-q` | Search query (can be repeated for multiple queries) | - |
| `-include-summary` | Include AI-generated summaries | off for single, on for multi |
| `-json` | Output in JSON format | false |
| `-stream` | Stream results for single queries only | false |
| `-workers` | Max concurrent workers (1-5) | 3 |
| `-timeout` | Total operation timeout | 3m |
| `-verbose`, `-v` | Enable verbose logging | false |

## Examples

```bash
# Basic search (no summary by default for single queries)
./search "What is Go programming?"

# Single query with summary
./search -include-summary "What is Go programming?"

# Compare multiple technologies (summaries included by default)
./search -q "Go" -q "Python" -q "Rust"

# Multiple queries without summaries
./search -q "Go" -q "Python" -include-summary=false

# Stream mode for single queries only
./search -stream "What is React?"

# JSON output for automation
./search -q "React" -q "Vue" -json

# Custom concurrency settings
./search -q "ML" -q "AI" -q "Deep Learning" -workers 2

# Verbose output (flags before positional query)
./search -v "What is Go programming?"

# Research workflow example
./search -q "Docker best practices" -q "Kubernetes deployment" -q "CI/CD pipelines" -workers 3
```

## Requirements

- Go 1.21 or later
- Google AI API credentials (see [setup guide](https://github.com/googleapis/go-genai))

## Installation

```bash
git clone <repository-url>
cd search
go build -o search
```

**Important:** When using positional arguments, flags must come before the query.
