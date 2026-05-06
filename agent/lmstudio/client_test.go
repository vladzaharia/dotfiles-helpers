package lmstudio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseParamsB(t *testing.T) {
	cases := map[string]float64{
		"gemma-4-31b-it":                            31,
		"gemma-4-e2b-it":                            2,
		"qwen3.5-35b-a3b-uncensored-hauhau@bf16":    35,
		"qwen3.6-35b-a3b":                           35,
		"unsloth/gemma-4-26b-a4b-it":                26,
		"mistral-7b-instruct-v0.3":                  7,
		"phi-4-mini-3.8b@q4":                        3.8,
		"text-embedding-nomic-1.5":                  0,
		"gemma-4-e4b-it":                            4,
		"llama3.3-70b-instruct":                     70,
	}
	for id, want := range cases {
		if got := ParseParamsB(id); got != want {
			t.Errorf("ParseParamsB(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestPickLargestLoaded(t *testing.T) {
	models := []Model{
		{ID: "small-3b", State: "loaded", CompatibilityType: "gguf"},
		{ID: "big-70b", State: "loaded", CompatibilityType: "gguf"},
		{ID: "huge-405b", State: "not-loaded", CompatibilityType: "gguf"},
		{ID: "remote-claude-3.5-sonnet", State: "loaded", CompatibilityType: "openai"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": models})
	}))
	defer srv.Close()

	c := &Client{URL: srv.URL}
	m, err := c.PickLargestLoaded()
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "big-70b" {
		t.Fatalf("picked %q, want big-70b", m.ID)
	}
}

func TestPickLargestLoadedFiltersLMLink(t *testing.T) {
	models := []Model{
		{ID: "remote-claude-200b", State: "loaded", CompatibilityType: "openai"},
		{ID: "tiny-1b", State: "loaded", CompatibilityType: "gguf"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": models})
	}))
	defer srv.Close()
	c := &Client{URL: srv.URL}
	m, err := c.PickLargestLoaded()
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "tiny-1b" {
		t.Fatalf("picked %q, want tiny-1b (LM Link must be filtered)", m.ID)
	}
}
