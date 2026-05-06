// Package lmstudio is a thin client for the local LM Studio REST API,
// used by `--local` to pick a suitable on-device model dynamically.
//
// We prefer LM Studio's extended `/api/v0/models` endpoint (richer
// metadata: state, quantization, max_context_length, compatibility_type)
// over the OpenAI-compatible `/v1/models` shape. Only models with a
// recognized local compatibility_type ("gguf" or "mlx") are considered;
// LM Link / cloud-routed entries are filtered out.
package lmstudio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Client is an LM Studio HTTP client.
type Client struct {
	URL  string        // e.g. "http://127.0.0.1:1234"
	HTTP *http.Client  // nil = default with 3s timeout
}

// Model mirrors the JSON shape of /api/v0/models entries.
type Model struct {
	ID                  string   `json:"id"`
	State               string   `json:"state"`
	Type                string   `json:"type"`
	Arch                string   `json:"arch"`
	Publisher           string   `json:"publisher"`
	Quantization        string   `json:"quantization"`
	CompatibilityType   string   `json:"compatibility_type"`
	MaxContextLength    int      `json:"max_context_length"`
	LoadedContextLength int      `json:"loaded_context_length"`
	Capabilities        []string `json:"capabilities"`
}

// IsLocal reports whether the model is a local file (not LM-Link / cloud).
func (m Model) IsLocal() bool {
	switch strings.ToLower(m.CompatibilityType) {
	case "gguf", "mlx", "safetensors":
		return true
	}
	return false
}

// IsLoaded reports whether the model is currently in memory.
func (m Model) IsLoaded() bool { return m.State == "loaded" }

// ParamsB returns the (largest) parameter count parsed out of the model id,
// in billions. For MoE models named like `qwen3.6-35b-a3b`, the 35b total
// is preferred over the 3b activation. Returns 0 when nothing parses.
func (m Model) ParamsB() float64 {
	return ParseParamsB(m.ID)
}

var paramRE = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)([bm])\b`)

// ParseParamsB scans an LM Studio model id and returns the largest
// parameter-count token (in billions). Tokens after a `@` (quant suffix)
// are ignored.
func ParseParamsB(id string) float64 {
	if at := strings.Index(id, "@"); at >= 0 {
		id = id[:at]
	}
	matches := paramRE.FindAllStringSubmatch(id, -1)
	var best float64
	for _, m := range matches {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(m[2]) {
		case "m":
			v = v / 1000
		case "b":
			// already in billions
		}
		if v > best {
			best = v
		}
	}
	return best
}

// Models hits `/api/v0/models` and returns the parsed list.
func (c *Client) Models() ([]Model, error) {
	httpc := c.HTTP
	if httpc == nil {
		httpc = &http.Client{Timeout: 3 * time.Second}
	}
	resp, err := httpc.Get(strings.TrimRight(c.URL, "/") + "/api/v0/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("lmstudio: %s", resp.Status)
	}
	var body struct {
		Data []Model `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

// PickLargestLoaded returns the loaded local model with the largest parsed
// parameter count. Models without a parseable size (and not loaded) are
// skipped. Returns an error when no candidate exists.
func (c *Client) PickLargestLoaded() (Model, error) {
	all, err := c.Models()
	if err != nil {
		return Model{}, err
	}
	var cands []Model
	for _, m := range all {
		if !m.IsLoaded() || !m.IsLocal() || m.ParamsB() == 0 {
			continue
		}
		cands = append(cands, m)
	}
	if len(cands) == 0 {
		return Model{}, fmt.Errorf("no loaded local models with parseable size")
	}
	sort.SliceStable(cands, func(i, j int) bool {
		return cands[i].ParamsB() > cands[j].ParamsB()
	})
	return cands[0], nil
}
