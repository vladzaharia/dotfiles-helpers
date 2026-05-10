package lmstudio

import (
	"sort"
	"strings"
)

// Tier categorizes a local model by approximate capability, mirroring the
// Anthropic family naming so the wizard can suggest a sensible default per
// tier (haiku=fast/cheap, sonnet=balanced, opus=heavy).
type Tier int

const (
	TierUnknown Tier = iota
	TierHaiku
	TierSonnet
	TierOpus
)

func (t Tier) String() string {
	switch t {
	case TierHaiku:
		return "haiku"
	case TierSonnet:
		return "sonnet"
	case TierOpus:
		return "opus"
	}
	return "unknown"
}

// AllTiers returns the tiers in ascending capability order.
func AllTiers() []Tier { return []Tier{TierHaiku, TierSonnet, TierOpus} }

// HasCapability reports whether the model advertises the given capability
// (case-insensitive). LM Studio capabilities include "tool_use", "vision",
// "reasoning", etc.
func (m Model) HasCapability(name string) bool {
	name = strings.ToLower(name)
	for _, c := range m.Capabilities {
		if strings.ToLower(c) == name {
			return true
		}
	}
	return false
}

// TierFor classifies a model by parameter count plus capability hints.
//
// Rules:
//   - >70B params → Opus
//   - >=30B params AND reasoning capability → Opus
//   - 14-70B params → Sonnet
//   - <14B params → Haiku
//   - unparseable size → Haiku (smallest fallback)
func TierFor(m Model) Tier {
	p := m.ParamsB()
	switch {
	case p > 70:
		return TierOpus
	case p >= 30 && m.HasCapability("reasoning"):
		return TierOpus
	case p >= 14:
		return TierSonnet
	case p > 0:
		return TierHaiku
	}
	return TierHaiku
}

// GroupByTier buckets local models by tier. Non-local entries (LM-Link /
// cloud-routed) are filtered out.
func GroupByTier(models []Model) map[Tier][]Model {
	out := map[Tier][]Model{}
	for _, m := range models {
		if !m.IsLocal() {
			continue
		}
		t := TierFor(m)
		out[t] = append(out[t], m)
	}
	for t := range out {
		ms := out[t]
		sort.SliceStable(ms, func(i, j int) bool {
			return ms[i].ParamsB() > ms[j].ParamsB()
		})
		out[t] = ms
	}
	return out
}

// Suggest picks the best model in `models` for `tier`. It prefers loaded
// models and, for Opus, reasoning-capable ones. Returns nil when the tier
// has no candidates.
func Suggest(models []Model, tier Tier) *Model {
	candidates := []Model{}
	for _, m := range models {
		if !m.IsLocal() {
			continue
		}
		if TierFor(m) != tier {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.IsLoaded() != b.IsLoaded() {
			return a.IsLoaded()
		}
		if tier == TierOpus {
			if a.HasCapability("reasoning") != b.HasCapability("reasoning") {
				return a.HasCapability("reasoning")
			}
		}
		return a.ParamsB() > b.ParamsB()
	})
	return &candidates[0]
}
