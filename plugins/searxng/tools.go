package searxng

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/mwantia/forge/pkg/plugins"
)

func (d *SearXNGDriver) GetLifecycle() plugins.Lifecycle {
	return d
}

func (d *SearXNGDriver) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	tools := make([]plugins.ToolDefinition, 0, len(d.config.Tools))
	for _, name := range d.config.Tools {
		def, ok := toolDefinitions[name]
		if !ok {
			d.log.Warn("Unknown tool in config, skipping", "tool", name)
			continue
		}
		if matchesFilter(def, filter) {
			tools = append(tools, def)
		}
	}

	return &plugins.ListToolsResponse{Tools: tools}, nil
}

func (d *SearXNGDriver) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	def, ok := toolDefinitions[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	if slices.Contains(d.config.Tools, name) {
		return &def, nil
	}
	return nil, fmt.Errorf("tool %q is not enabled", name)
}

func (d *SearXNGDriver) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	var errs []string
	switch req.Tool {
	case "web_search":
		if v, ok := req.Arguments["query"]; !ok || v == "" {
			errs = append(errs, `"query" is required`)
		}
	case "web_fetch":
		if v, ok := req.Arguments["url"]; !ok || v == "" {
			errs = append(errs, `"url" is required`)
		}
	default:
		errs = append(errs, fmt.Sprintf("unknown tool %q", req.Tool))
	}
	return &plugins.ValidateResponse{Valid: len(errs) == 0, Errors: errs}, nil
}

func (d *SearXNGDriver) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	enabled := false
	for _, t := range d.config.Tools {
		if t == req.Tool {
			enabled = true
			break
		}
	}
	if !enabled {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("tool '%s' is not enabled in searxng configuration", req.Tool),
			IsError: true,
		}, nil
	}

	switch req.Tool {
	case "web_search":
		return d.execSearch(ctx, req.Arguments)
	case "web_fetch":
		return d.execFetch(ctx, req.Arguments)
	default:
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("unknown tool: %s", req.Tool),
			IsError: true,
		}, nil
	}
}

// searxngResult is a single result from the SearXNG JSON response.
type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Engine  string `json:"engine"`
}

// searxngResponse is the top-level SearXNG JSON response.
type searxngResponse struct {
	Query           string          `json:"query"`
	NumberOfResults int             `json:"number_of_results"`
	Results         []searxngResult `json:"results"`
}

func (d *SearXNGDriver) execSearch(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &plugins.ExecuteResponse{Result: "query is required", IsError: true}, nil
	}

	maxResults := d.config.MaxResults
	if n, ok := args["num_results"]; ok {
		switch v := n.(type) {
		case float64:
			maxResults = int(v)
		case int:
			maxResults = v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				maxResults = i
			}
		}
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")

	if cats, ok := args["categories"].(string); ok && cats != "" {
		params.Set("categories", cats)
	}
	if lang, ok := args["language"].(string); ok && lang != "" {
		params.Set("language", lang)
	} else {
		params.Set("language", "en")
	}

	searchURL := d.config.Address + "/search?" + params.Encode()
	d.log.Debug("Executing web_search", "url", searchURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to build request: %v", err), IsError: true}, nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("search request failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("search returned status %d", resp.StatusCode),
			IsError: true,
		}, nil
	}

	var parsed searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to parse response: %v", err), IsError: true}, nil
	}

	results := parsed.Results
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	type resultEntry struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Engine  string `json:"engine"`
	}
	entries := make([]resultEntry, 0, len(results))
	for _, r := range results {
		entries = append(entries, resultEntry(r))
	}

	d.log.Debug("web_search completed", "query", query, "results", len(entries))
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"query":             query,
			"number_of_results": parsed.NumberOfResults,
			"results":           entries,
		},
	}, nil
}

func (d *SearXNGDriver) execFetch(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	rawURL, ok := args["url"].(string)
	if !ok || rawURL == "" {
		return &plugins.ExecuteResponse{Result: "url is required", IsError: true}, nil
	}

	d.log.Debug("Executing web_fetch", "url", rawURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("invalid URL: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "forge-searxng/0.1")

	resp, err := d.client.Do(req)
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("fetch failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to read response: %v", err), IsError: true}, nil
	}

	contentType := resp.Header.Get("Content-Type")
	var content string
	if strings.Contains(contentType, "text/html") {
		content = extractText(string(body))
	} else {
		content = string(body)
	}

	d.log.Debug("web_fetch completed", "url", rawURL, "status", resp.StatusCode, "bytes", len(content))
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"url":          rawURL,
			"status_code":  resp.StatusCode,
			"content_type": contentType,
			"content":      content,
		},
	}, nil
}

// extractText extracts plain text from an HTML document.
func extractText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "head", "noscript":
				return
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				buf.WriteString(text)
				buf.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return buf.String()
}

func matchesFilter(def plugins.ToolDefinition, f plugins.ListToolsFilter) bool {
	if def.Deprecated && !f.Deprecated {
		return false
	}
	if f.Prefix != "" && !strings.HasPrefix(def.Name, f.Prefix) {
		return false
	}
	if len(f.Tags) > 0 {
		for _, want := range f.Tags {
			for _, have := range def.Tags {
				if have == want {
					goto tagMatched
				}
			}
		}
		return false
	tagMatched:
	}
	return true
}
