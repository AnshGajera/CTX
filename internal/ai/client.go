package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client talks to the ctx-ml FastAPI sidecar (optional).
type Client struct {
	base string
	http *http.Client
}

// NewClient creates a client; base like http://localhost:8001.
func NewClient(base string) *Client {
	return &Client{base: base, http: &http.Client{Timeout: 15 * time.Second}}
}

// SearchResult is one semantic hit.
type SearchResult struct {
	Text   string  `json:"text"`
	Source string  `json:"source"`
	Score  float64 `json:"score"`
}

// SemanticSearch tries remote ML, returns (nil, err) when unavailable.
func (c *Client) SemanticSearch(query string, chunks []Chunk, topK int) ([]SearchResult, error) {
	body, _ := json.Marshal(map[string]any{"query": query, "chunks": chunks, "top_k": topK})
	resp, err := c.http.Post(c.base+"/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ml sidecar unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ml search status %d", resp.StatusCode)
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode ml results: %w", err)
	}
	return out.Results, nil
}

// HybridSearch prefers ML but falls back to TF-IDF.
func HybridSearch(mlBase, query string, chunks []Chunk, topK int) []RankedChunk {
	if mlBase != "" {
		if res, err := NewClient(mlBase).SemanticSearch(query, chunks, topK); err == nil && len(res) > 0 {
			var out []RankedChunk
			for _, r := range res {
				out = append(out, RankedChunk{Chunk: Chunk{Text: r.Text, Source: r.Source}, Score: r.Score})
			}
			return out
		}
	}
	return RankTFIDF(chunks, query, topK)
}
