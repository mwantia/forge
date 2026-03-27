package searxng

import "github.com/mwantia/forge/pkg/plugins"

var toolDefinitions = map[string]plugins.ToolDefinition{
	"web_search": {
		Name:        "web_search",
		Description: "Search the web using SearXNG and return a list of results with titles, URLs, and content snippets",
		Tags:        []string{"web", "search"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query",
				},
				"num_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (defaults to plugin max_results setting)",
				},
				"categories": map[string]any{
					"type":        "string",
					"description": "Comma-separated list of search categories (e.g. 'general,news,images')",
				},
				"language": map[string]any{
					"type":        "string",
					"description": "Language code for results (e.g. 'en', 'de'). Defaults to 'en'.",
				},
			},
			"required": []string{"query"},
		},
	},
	"web_fetch": {
		Name:        "web_fetch",
		Description: "Fetch the content of a web page and return its text",
		Tags:        []string{"web", "fetch"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to fetch",
				},
			},
			"required": []string{"url"},
		},
	},
}
