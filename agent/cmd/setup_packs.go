package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/huh"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// noDetect is set by the global --no-detect flag (registered in
// root.go) and read by dispatch logic to bypass project auto-detection.
var noDetect bool

// PromptProjectSetup is the per-directory first-run wizard. It runs
// when CWD has no `.agent-helper.toml` AND dispatch is interactive AND
// --no-detect isn't set. Pre-populates with auto-detected isolation +
// packs; user can adjust via ↑ before pressing enter.
//
// On submit, writes `.agent-helper.toml` in projectDir (so
// findProjectConfig picks it up on subsequent runs).
//
// Returns (chosenIsolation, chosenPacks, saved, err). saved=true means
// the file was written; false means user esc'd or submitted "don't
// save" (we use detected values for this dispatch only).
func PromptProjectSetup(projectDir string, baseline []string, sharedExists bool, sharedMounts []string) (string, []string, bool, error) {
	iso := orb.DetectProjectIsolation(projectDir, sharedExists, sharedMounts)
	detectedPacks, evidence := orb.DetectProjectPacks(projectDir)
	allPacks, err := loadAllPackOptions()
	if err != nil {
		return "", nil, false, err
	}

	// Pre-checked: baseline ∪ detected.
	pre := map[string]bool{}
	for _, p := range baseline {
		pre[p] = true
	}
	for _, p := range detectedPacks {
		pre[p] = true
	}
	pre["base"] = true // base is always required

	chosen := []string{}
	for _, p := range allPacks {
		if pre[p] {
			chosen = append(chosen, p)
		}
	}

	// Per-pack focused infobox. Detected packs include their evidence
	// in the description; non-detected packs get the pack's own help
	// text.
	evidenceByPack := map[string]DetectionEvidence{}
	for _, ev := range evidence {
		evidenceByPack[ev.Pack] = DetectionEvidence(ev)
	}

	packOpts := make([]huh.Option[string], 0, len(allPacks))
	for _, p := range allPacks {
		label := p
		if ev, ok := evidenceByPack[p]; ok && len(ev.Files) > 0 {
			label = p + "  · " + strings.Join(uniq(ev.Files), ", ") + " found"
		}
		if p == "base" {
			label = p + "  · core utilities (required)"
		}
		packOpts = append(packOpts, huh.NewOption(label, p))
	}

	isolation := iso.Mode
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(output.Style.Bold.Render("Setup for "+friendlyPath(projectDir))),
			huh.NewSelect[string]().
				Title("Isolation").
				DescriptionFunc(func() string {
					return output.InfoBox(isolation + "\n" + isolationReasonFor(isolation, iso))
				}, &isolation).
				Options(
					huh.NewOption("none — run on host", "none"),
					huh.NewOption("full — dedicated ephemeral VM", "full"),
					huh.NewOption("shared — long-lived shared VM", "shared"),
				).
				Value(&isolation),
			huh.NewMultiSelect[string]().
				Title("Packs").
				Description("Pre-installed software in the VM. Auto-detected packs are pre-checked.").
				Options(packOpts...).
				Value(&chosen),
		).WithHideFunc(func() bool { return false }),
	)
	if err := runForm(form); err != nil {
		return "", nil, false, err
	}

	// Always include "base" — config validation downstream depends on
	// it being present.
	if !contains(chosen, "base") {
		chosen = append([]string{"base"}, chosen...)
	}

	if err := saveProjectConfig(projectDir, isolation, chosen); err != nil {
		return isolation, chosen, false, fmt.Errorf("save .agent-helper.toml: %w", err)
	}
	return isolation, chosen, true, nil
}

// DetectionEvidence is re-exported here so callers in this package
// don't have to import agent/orb just for the type. (Same shape; the
// alias is for ergonomics only.)
type DetectionEvidence = orb.DetectionEvidence

// loadAllPackOptions returns the names of all packs known to the
// orb package, sorted alphabetically. base is always first.
func loadAllPackOptions() ([]string, error) {
	packs, err := orb.LoadPacks()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(packs))
	for n := range packs {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "base" {
			return true
		}
		if names[j] == "base" {
			return false
		}
		return names[i] < names[j]
	})
	return names, nil
}

// isolationReasonFor returns the human-readable explanation for the
// currently-focused isolation option. The auto-detected option carries
// the orb.DetectProjectIsolation reason; others get a generic blurb.
func isolationReasonFor(focused string, detected orb.IsolationDecision) string {
	if focused == detected.Mode {
		return detected.Reason
	}
	switch focused {
	case "none":
		return "Run on host. Full filesystem access. Fastest. Use only for trusted prompts."
	case "shared":
		return "Long-lived per-provider VM. Faster startup but state persists across sessions."
	case "full":
		return "Dedicated ephemeral VM per task. Strong isolation. ~10s startup. Auto-cleanup."
	}
	return ""
}

// saveProjectConfig writes <projectDir>/.agent-helper.toml with just
// the isolation and pack overrides. Existing fields in that file are
// preserved (the file is loaded, mutated, re-written).
func saveProjectConfig(projectDir, isolation string, packs []string) error {
	path := filepath.Join(projectDir, projectConfigName)
	var cfg Config
	if data, err := os.ReadFile(path); err == nil {
		_ = toml.Unmarshal(data, &cfg)
	}
	cfg.Defaults.Isolated = isolation
	cfg.Orb.DefaultPacks = packs
	f, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(f).Encode(&cfg); err != nil {
		f.Close()
		os.Remove(path + ".tmp")
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path + ".tmp")
		return err
	}
	return os.Rename(path+".tmp", path)
}

// friendlyPath shortens an absolute path with ~ when it's under $HOME.
func friendlyPath(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func uniq(ss []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
