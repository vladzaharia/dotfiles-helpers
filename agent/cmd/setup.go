package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"golang.org/x/term"

	"github.com/vladzaharia/dotfiles-helpers/agent/lmstudio"
	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	"github.com/vladzaharia/dotfiles-helpers/internal/config"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// runForm runs a huh form and converts user-abort (Ctrl-C / Esc) into
// a clean process exit so callers don't have to check for ErrUserAborted
// at every form.Run() site.
func runForm(form *huh.Form) error {
	err := form.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Fprintln(os.Stderr)
		output.Warn("Setup interrupted; nothing further was saved.")
		os.Exit(130)
	}
	return err
}

var setupNonInteractive bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard",
	RunE:  runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&setupNonInteractive, "non-interactive", false,
		"Write defaults without prompting (also auto-enabled when stdin is not a TTY)")
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	interactive := !setupNonInteractive && term.IsTerminal(int(os.Stdin.Fd()))

	fmt.Println()
	output.Info("Agent Helper Setup")
	fmt.Println()

	// ── Detect host providers ────────────────────────────────────────
	fmt.Println("  Detecting providers...")
	statuses := []provider.Status{
		provider.DetectClaude(),
		provider.DetectCodex(),
		provider.DetectLMStudio(cfg.Local.URL),
	}
	if ollama := provider.DetectOllama(); ollama.Installed {
		statuses = append(statuses, ollama)
	}
	fmt.Println()
	provider.PrintStatus(statuses)
	fmt.Println()

	if !interactive {
		output.Info("Non-interactive mode — writing defaults.")
		return saveAndReport(&cfg)
	}

	// ── Step A: Defaults ─────────────────────────────────────────────
	isolationOptions := []huh.Option[string]{
		huh.NewOption("none — run on host", "none"),
		huh.NewOption("shared — long-lived per-provider VM", "shared"),
		huh.NewOption("full — dedicated ephemeral VM (recommended)", "full"),
	}
	defaultsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default agent").
				Options(
					huh.NewOption("Claude Code", "claude"),
					huh.NewOption("Codex CLI", "codex"),
				).
				Value(&cfg.Defaults.Agent),
		),
	)
	if err := runForm(defaultsForm); err != nil {
		return err
	}

	// Isolation is macOS-only (OrbStack) — skip on Linux and force "none".
	if runtime.GOOS == "darwin" {
		isoForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Default isolation mode").
					Description("Where should agents run by default?").
					Options(isolationOptions...).
					Value(&cfg.Defaults.Isolated),
			),
		)
		if err := runForm(isoForm); err != nil {
			return err
		}
	} else {
		cfg.Defaults.Isolated = "none"
		output.Info("Isolation modes require OrbStack (macOS only) — defaulting to none.")
	}

	// ── Step B: OrbStack auto-install (macOS, isolation != none) ─────
	if runtime.GOOS == "darwin" && cfg.Defaults.Isolated != "none" {
		if err := promptOrbStackInstall(ctx); err != nil {
			// Install failed AND user wanted a VM mode. Downgrade isolation
			// to "none" so we don't save a config that promises a VM
			// runtime that will fail at first dispatch. The user can
			// re-run setup once OrbStack is sorted out.
			output.Warn("OrbStack setup: %v", err)
			output.Warn("Falling back to isolation=none for now. Install OrbStack manually and re-run `agent-helper setup orb`.")
			cfg.Defaults.Isolated = "none"
		} else if !orb.IsInstalled() {
			// User declined the install prompt — same fallback so the
			// saved config stays consistent with what's actually possible.
			cfg.Defaults.Isolated = "none"
		}
	}

	// ── Step C: Packs + mount roots (macOS, isolation != none) ───────
	if runtime.GOOS == "darwin" && cfg.Defaults.Isolated != "none" {
		if err := promptPacksAndMounts(&cfg); err != nil {
			return err
		}
	}

	// ── Step D: Provider binary overrides ────────────────────────────
	if err := promptProviderOverrides(&cfg); err != nil {
		return err
	}

	// ── Step E: LM Studio model picker ───────────────────────────────
	if err := promptLMStudio(ctx, &cfg); err != nil {
		output.Warn("LM Studio setup: %v", err)
	}

	// ── Save before token flow so we don't lose work ─────────────────
	if err := saveAndReport(&cfg); err != nil {
		return err
	}

	// ── Step F: Claude credentials for VM use ────────────────────────
	fmt.Println()
	fmt.Println("  Claude credentials (for VM auth)")
	fmt.Println("  ────────────────────────────────")
	printClaudeAuthSummary()

	if needsClaudeAuthSetup() {
		if err := promptClaudeAuth(); err != nil {
			output.Warn("Claude auth: %v", err)
		}
	}

	fmt.Println()
	output.Success("Setup complete. Try `agent-helper doctor` to verify.")
	fmt.Println()
	return nil
}

func saveAndReport(cfg *Config) error {
	if err := config.Save("agent-helper", cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	output.Success("Config saved to %s", config.Path("agent-helper"))
	return nil
}

func promptOrbStackInstall(ctx context.Context) error {
	if orb.IsInstalled() {
		fmt.Println(output.StatusOK("OrbStack", "already installed"))
		return nil
	}
	var install bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("OrbStack isn't installed").
				Description("Isolation modes require OrbStack. Install it now?").
				Affirmative("Install").
				Negative("Skip").
				Value(&install),
		),
	)
	if err := runForm(form); err != nil {
		return err
	}
	if !install {
		output.Warn("OrbStack skipped — VM modes will fall back to host execution.")
		return nil
	}
	if err := orb.EnsureInstalled(ctx, os.Stderr); err != nil {
		return err
	}
	if err := orb.WaitForDaemon(ctx, 90*time.Second); err != nil {
		output.Warn("OrbStack daemon: %v", err)
	}
	output.Success("OrbStack installed.")
	return nil
}

func promptPacksAndMounts(cfg *Config) error {
	packs, err := orb.LoadPacks()
	if err != nil {
		return fmt.Errorf("load packs: %w", err)
	}
	packOptions := make([]huh.Option[string], 0, len(packs))
	names := make([]string, 0, len(packs))
	for name := range packs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := packs[name]
		label := name
		if p.Description != "" {
			label = fmt.Sprintf("%s — %s", name, p.Description)
		}
		packOptions = append(packOptions, huh.NewOption(label, name))
	}

	mounts := strings.Join(cfg.Orb.MountRoots, ",")
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Default packs for shared/prebuilt VMs").
				Description("Pre-installed in shared VMs and as a base for dedicated VMs.").
				Options(packOptions...).
				Value(&cfg.Orb.DefaultPacks),
			huh.NewInput().
				Title("Mount roots").
				Description("Comma-separated host paths mounted into VMs (e.g. ~/Repos,~/Code).").
				Value(&mounts),
		),
	)
	if err := runForm(form); err != nil {
		return err
	}
	cfg.Orb.MountRoots = splitCSV(mounts)
	return nil
}

func promptProviderOverrides(cfg *Config) error {
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}
	for _, p := range provider.All() {
		name := p.Name()
		st := p.DetectHost()
		detected := ""
		if st.Installed {
			if path, err := iexec.FindRealBinary(p.Binary()); err == nil {
				detected = path
			}
		}

		current := cfg.Providers[name].BinaryPath
		if current == "" {
			current = detected
		}

		desc := "Leave blank to use whichever `" + p.Binary() + "` is on $PATH."
		if detected != "" {
			desc = "Detected: " + detected + ". Override or leave as-is."
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(fmt.Sprintf("Path to %s binary", p.DisplayName())).
					Description(desc).
					Value(&current),
			),
		)
		if err := runForm(form); err != nil {
			return err
		}
		entry := cfg.Providers[name]
		entry.BinaryPath = strings.TrimSpace(current)
		cfg.Providers[name] = entry
	}
	return nil
}

func promptLMStudio(ctx context.Context, cfg *Config) error {
	urlValue := cfg.Local.URL
	urlForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("LM Studio URL").
				Description("HTTP endpoint for `--local` mode. Default is http://127.0.0.1:1234.").
				Validate(validateLMStudioURL).
				Value(&urlValue),
		),
	)
	if err := runForm(urlForm); err != nil {
		return err
	}
	cfg.Local.URL = strings.TrimSpace(urlValue)

	client := &lmstudio.Client{URL: cfg.Local.URL}
	models, err := client.Models()
	if err != nil || len(models) == 0 {
		var skip bool
		msg := "LM Studio isn't reachable at that URL."
		if err == nil {
			msg = "LM Studio reachable but reports no models."
		}
		skipForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(msg).
					Description("Skip local model setup for now? You can re-run `agent-helper setup` later.").
					Affirmative("Skip").
					Negative("Retry").
					Value(&skip),
			),
		)
		if formErr := runForm(skipForm); formErr != nil {
			return formErr
		}
		if skip {
			return nil
		}
		// Poll up to 3 times at 1s intervals so the user has a window to
		// start LM Studio or load a model between Retry and the actual
		// next probe.
		for i := 0; i < 3; i++ {
			time.Sleep(1 * time.Second)
			models, err = client.Models()
			if err == nil && len(models) > 0 {
				break
			}
		}
		if err != nil || len(models) == 0 {
			output.Warn("Still no models — skipping. Use `agent-helper config edit` to set later.")
			return nil
		}
	}

	grouped := lmstudio.GroupByTier(models)

	for _, tier := range lmstudio.AllTiers() {
		bucket := grouped[tier]
		opts := []huh.Option[string]{huh.NewOption("(skip — no model for this tier)", "")}
		for _, m := range bucket {
			label := fmt.Sprintf("%s  %.0fB %s%s",
				m.ID, m.ParamsB(), m.CompatibilityType, capabilitySuffix(m))
			if m.IsLoaded() {
				label = "● " + label
			} else {
				label = "○ " + label
			}
			opts = append(opts, huh.NewOption(label, m.ID))
		}

		var pick string
		if suggested := lmstudio.Suggest(models, tier); suggested != nil {
			pick = suggested.ID
		}
		// Honor any existing config value.
		switch tier {
		case lmstudio.TierHaiku:
			if cfg.Local.Models.Haiku != "" {
				pick = cfg.Local.Models.Haiku
			}
		case lmstudio.TierSonnet:
			if cfg.Local.Models.Sonnet != "" {
				pick = cfg.Local.Models.Sonnet
			}
		case lmstudio.TierOpus:
			if cfg.Local.Models.Opus != "" {
				pick = cfg.Local.Models.Opus
			}
		}
		// Stale-pick safety: if the previously-saved model is no longer in
		// LM Studio's current options, drop it so huh doesn't render a
		// broken pre-selection.
		if pick != "" && !optionsContain(opts, pick) {
			pick = ""
			if suggested := lmstudio.Suggest(models, tier); suggested != nil {
				pick = suggested.ID
			}
		}

		title := fmt.Sprintf("Pick a %s-tier model", tier.String())
		desc := tierDescription(tier)
		if len(bucket) == 0 {
			desc = "No loaded local models match this tier. " + desc
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(title).
					Description(desc).
					Options(opts...).
					Value(&pick),
			),
		)
		if err := runForm(form); err != nil {
			return err
		}
		switch tier {
		case lmstudio.TierHaiku:
			cfg.Local.Models.Haiku = pick
		case lmstudio.TierSonnet:
			cfg.Local.Models.Sonnet = pick
		case lmstudio.TierOpus:
			cfg.Local.Models.Opus = pick
		}
	}

	// Preserve single-default behavior: prefer Sonnet, fall back to whichever
	// tier has a value.
	if cfg.Local.DefaultModel == "" {
		for _, m := range []string{cfg.Local.Models.Sonnet, cfg.Local.Models.Haiku, cfg.Local.Models.Opus} {
			if m != "" {
				cfg.Local.DefaultModel = m
				break
			}
		}
	}
	return nil
}

// optionsContain reports whether the slice of huh select options
// contains an option whose Value equals v. Used to detect stale model
// IDs (saved in config but no longer offered by LM Studio).
func optionsContain(opts []huh.Option[string], v string) bool {
	for _, o := range opts {
		if o.Value == v {
			return true
		}
	}
	return false
}

// validateLMStudioURL ensures the input parses as an HTTP/HTTPS URL with
// a host, and warns about the common HTTPS-on-localhost mistake (LM
// Studio serves plain HTTP locally; a stray "https://" otherwise hides
// itself as a TLS handshake error later).
func validateLMStudioURL(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("URL is required")
	}
	u, err := neturl.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http:// or https://")
	}
	if u.Host == "" {
		return fmt.Errorf("URL needs a host (e.g. 127.0.0.1:1234)")
	}
	host := u.Hostname()
	if u.Scheme == "https" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return fmt.Errorf("LM Studio serves plain HTTP locally; use http:// not https:// for %s", host)
	}
	return nil
}

func tierDescription(t lmstudio.Tier) string {
	switch t {
	case lmstudio.TierHaiku:
		return "Fast, lightweight (<14B params)."
	case lmstudio.TierSonnet:
		return "Balanced default (14–70B params)."
	case lmstudio.TierOpus:
		return "Heavy / reasoning models (>70B or 30B+ with reasoning)."
	}
	return ""
}

func capabilitySuffix(m lmstudio.Model) string {
	if len(m.Capabilities) == 0 {
		return ""
	}
	return " [" + strings.Join(m.Capabilities, ",") + "]"
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func promptClaudeAuth() error {
	options := []huh.Option[string]{
		huh.NewOption("Generate a long-lived OAuth token (recommended)", "token"),
	}
	if runtime.GOOS == "darwin" && hasKeychainEntry() {
		options = append(options, huh.NewOption("Sync from macOS Keychain", "keychain"))
	}
	options = append(options, huh.NewOption("Skip", "skip"))

	choice := "token"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Claude isn't fully set up for VM use yet").
				Options(options...).
				Value(&choice),
		),
	)
	if err := runForm(form); err != nil {
		return err
	}
	switch choice {
	case "token":
		return runClaudeSetupTokenInteractive()
	case "keychain":
		return runKeychainExport()
	}
	return nil
}

// runClaudeSetupTokenInteractive runs `claude setup-token` with stdio attached
// so the user can complete the browser OAuth flow, captures the printed
// token, and persists it to secrets.toml.
func runClaudeSetupTokenInteractive() error {
	binPath, err := iexec.FindRealBinary("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w", err)
	}
	// Pre-flight: ensure secrets file path is writable before running the
	// 30-second browser OAuth flow. Catches permission/disk issues early
	// so the user doesn't authorize and then lose the token.
	if err := ensureSecretsWritable(); err != nil {
		return fmt.Errorf("can't save secrets to %s: %w", secretsPath(), err)
	}
	output.Info("Running `claude setup-token` — a browser will open. Authorize and copy the token.")
	cmd := exec.Command(binPath, "setup-token")
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	var captured bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &captured)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude setup-token: %w", err)
	}

	tokenRE := regexp.MustCompile(`sk-ant-oat\S+`)
	token := tokenRE.FindString(captured.String())
	if token == "" {
		var manual string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Token not auto-detected").
					Description("Paste the token printed above (starts with sk-ant-oat).").
					EchoMode(huh.EchoModePassword).
					Validate(func(s string) error {
						s = strings.TrimSpace(s)
						if s == "" {
							return fmt.Errorf("token is required")
						}
						if !tokenRE.MatchString(s) {
							return fmt.Errorf("doesn't look like a Claude OAuth token (expected sk-ant-oat...)")
						}
						return nil
					}).
					Value(&manual),
			),
		)
		if formErr := runForm(form); formErr != nil {
			return formErr
		}
		// Validation guarantees a match; extract just the token portion in
		// case the user pasted surrounding whitespace or context.
		token = tokenRE.FindString(manual)
	}
	if token == "" {
		return fmt.Errorf("no token captured")
	}

	s, _ := LoadSecrets()
	s.ClaudeOAuthToken = token
	if err := SaveSecrets(s); err != nil {
		return err
	}
	output.Success("Token saved to %s (mode 0600)", secretsPath())
	output.Info("agent-helper will forward CLAUDE_CODE_OAUTH_TOKEN to every VM session.")
	return nil
}

func needsClaudeAuthSetup() bool {
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" {
		return false
	}
	if st, err := os.Stat(credentialsPath()); err == nil && !st.IsDir() {
		return false
	}
	return true
}
