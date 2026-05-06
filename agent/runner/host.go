package runner

import (
	"fmt"
	"os"

	"github.com/vladzaharia/dotfiles-helpers/agent/lmstudio"
	"github.com/vladzaharia/dotfiles-helpers/agent/provider"
	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// HostRunner runs the provider's binary directly on the host, replacing
// the current process via syscall.Exec — matching pre-rearchitecture
// behavior.
type HostRunner struct{}

func (HostRunner) Run(p provider.Provider, passthrough []string, opts Options) error {
	if missing := iexec.ValidateDeps(p.Binary()); len(missing) > 0 {
		return fmt.Errorf("%s not found — %s", p.DisplayName(), p.InstallHint())
	}

	finalArgs := append([]string{}, p.DefaultArgs()...)
	if opts.Local {
		if !p.SupportsLocal() {
			return fmt.Errorf("--local is not supported for %s", p.DisplayName())
		}
		if opts.LocalURL == "" {
			return fmt.Errorf("--local requires LM Studio URL configured (run `agent-helper setup`)")
		}
		os.Setenv("ANTHROPIC_BASE_URL", opts.LocalURL)
		os.Setenv("ANTHROPIC_API_KEY", "lmstudio")

		model := opts.LocalModel
		if model == "" {
			c := &lmstudio.Client{URL: opts.LocalURL}
			pick, err := c.PickLargestLoaded()
			if err != nil {
				return fmt.Errorf("auto-pick local model: %w (load a model in LM Studio or set local.default_model)", err)
			}
			model = pick.ID
			output.Info("Auto-picked local model: %s (%.0fB params)", model, pick.ParamsB())
		}
		finalArgs = append(finalArgs, "--model", model)
		output.Info("Launching %s (local) via %s", p.DisplayName(), opts.LocalURL)
	} else {
		output.Info("Launching %s", p.DisplayName())
	}
	finalArgs = append(finalArgs, passthrough...)

	return iexec.Exec(p.Binary(), finalArgs)
}
