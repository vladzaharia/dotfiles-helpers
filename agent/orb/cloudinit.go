package orb

import "fmt"

// BootstrapPlan is the rendered output for a given (agent, packs) selection.
type BootstrapPlan struct {
	Yaml    []byte   // cloud-init for `orb create --user-data`
	Version string   // content hash, also written to /etc/agent-helper/bootstrap.version
	Runcmd  []string // ordered shell statements (idempotent), used for live reseed
}

// RenderCloudInit assembles a cloud-init YAML for the given agent and
// pack set. The agent's own pack name is auto-included if it isn't
// already in `wantedPacks`.
func RenderCloudInit(agent string, wantedPacks []string) (BootstrapPlan, error) {
	all, err := LoadPacks()
	if err != nil {
		return BootstrapPlan{}, fmt.Errorf("load packs: %w", err)
	}
	// Always include dev-cli (modern CLI tools) and the agent's own pack.
	wanted := append([]string{"dev-cli", agent}, wantedPacks...)
	resolved, err := Resolve(uniqueStrings(wanted), all)
	if err != nil {
		return BootstrapPlan{}, err
	}
	yaml, version, runcmd := Compose(resolved, agent)
	return BootstrapPlan{Yaml: yaml, Version: version, Runcmd: runcmd}, nil
}

// AvailablePacks returns the loaded packs map (for `agent-helper packs list`).
func AvailablePacks() (map[string]*Pack, error) { return LoadPacks() }

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
