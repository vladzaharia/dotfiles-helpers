package runner

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/vladzaharia/dotfiles-helpers/agent/orb"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// provisionGuard wraps the create + wait pair so SIGINT / SIGTERM during
// provisioning cancels both calls and triggers cleanup of the partial VM.
//
// The signal interception is scoped to this function via signal.NotifyContext.
// On success the deferred stop() releases it, so Ctrl+C during the
// subsequent SSH session is delivered through the default Go signal
// handling — claude in the VM handles it, SSH proxy closes, the runner
// exits cleanly without trying to delete the (still-running) VM.
//
// `create` and `wait` should both honor ctx.Done() (CreateContext /
// WaitCloudInitContext do this).
func provisionGuard(name string, create, wait func(context.Context) error) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := create(ctx); err != nil {
		cleanupPartialVM(name, err)
		return err
	}
	if err := wait(ctx); err != nil {
		cleanupPartialVM(name, err)
		return err
	}
	return nil
}

// cleanupPartialVM deletes a VM that failed to provision (or whose
// provisioning was cancelled by the user). It also forgets the VM from
// agent-helper's state directory so doctor/prune don't see ghosts.
//
// Both operations are best-effort — if they fail (e.g. orb daemon also
// dying), we don't surface a secondary error.
func cleanupPartialVM(name string, cause error) {
	if errors.Is(cause, context.Canceled) {
		output.Warn("Interrupted; deleting partial VM %q", name)
	} else {
		output.Warn("Provisioning failed for %q (%v); cleaning up", name, cause)
	}
	_ = orb.Delete(name)
	_ = state.ForgetVM(name)
}
