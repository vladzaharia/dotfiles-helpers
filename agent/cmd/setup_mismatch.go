package cmd

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/vladzaharia/dotfiles-helpers/agent/state"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

// MismatchAction is the user's choice when a `shared` dispatch needs
// packs the shared VM doesn't have.
type MismatchAction int

const (
	// MismatchAddToShared re-provisions the shared VM with the union of
	// existing + requested packs. Caller deletes the VM and lets the
	// next provision step re-create it; the new pack list propagates
	// via opts.ExtraPacks at that time.
	MismatchAddToShared MismatchAction = iota
	// MismatchUseFull switches THIS dispatch to ModeDedicated (full
	// isolation) without changing any saved config. The next dispatch
	// from this directory uses whatever .agent-helper.toml says.
	MismatchUseFull
	// MismatchSaveFull persists [defaults] isolated = "full" to the
	// project's .agent-helper.toml so this directory uses dedicated
	// VMs going forward.
	MismatchSaveFull
	// MismatchCancel aborts dispatch entirely.
	MismatchCancel
)

// MissingPacks returns the packs in `requested` that aren't present in
// the shared VM `vmName` according to the state record. Returns nil
// when there's no record (treat as "no info, don't prompt") or when
// nothing is missing.
func MissingPacks(vmName string, requested []string) []string {
	v, err := state.LoadVM(vmName)
	if err != nil || v == nil || len(v.Packs) == 0 {
		// No record means we can't say anything — treat as "no
		// mismatch" so we don't prompt on first-ever provision.
		return nil
	}
	have := map[string]bool{}
	for _, p := range v.Packs {
		have[p] = true
	}
	var missing []string
	for _, p := range requested {
		if !have[p] {
			missing = append(missing, p)
		}
	}
	return missing
}

// PromptSharedMismatch surfaces the recovery options when a `shared`
// dispatch's packs aren't all in the shared VM. Two contexts:
//
//   - First-use of a directory (form just submitted): "Save Full"
//     persists isolated=full to the project config.
//   - Subsequent dispatch where mismatch arises (e.g. project added a
//     pack signal): user can save Full or use it just for this run.
//
// In non-interactive contexts, the caller should fall back silently to
// MismatchUseFull (this function should not be called there).
func PromptSharedMismatch(vmName string, missing []string, projectDir string) (MismatchAction, error) {
	if len(missing) == 0 {
		return MismatchUseFull, nil
	}

	body := strings.Join([]string{
		"The shared VM (" + vmName + ") doesn't have these packs installed:",
		"  " + strings.Join(missing, ", "),
		"",
		"How would you like to proceed?",
	}, "\n")

	options := []huh.Option[string]{
		huh.NewOption("Add to shared VM (re-provisions, ~30s)", "add"),
		huh.NewOption("Spin up an independent VM for this project (just this run)", "use-full"),
	}
	if projectDir != "" {
		options = append(options,
			huh.NewOption("Spin up an independent VM and save the choice for this project", "save-full"))
	}
	options = append(options, huh.NewOption("Cancel", "cancel"))

	choice := "add"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(output.Style.Bold.Render("Pack mismatch")).
				Description(body),
			huh.NewSelect[string]().
				Title("").
				DescriptionFunc(func() string {
					return mismatchInfobox(choice)
				}, &choice).
				Options(options...).
				Value(&choice),
		),
	)
	if err := runForm(form); err != nil {
		return MismatchCancel, err
	}
	switch choice {
	case "add":
		return MismatchAddToShared, nil
	case "use-full":
		return MismatchUseFull, nil
	case "save-full":
		return MismatchSaveFull, nil
	}
	return MismatchCancel, nil
}

// mismatchInfobox returns the focus-aware infobox copy for each option.
func mismatchInfobox(focused string) string {
	switch focused {
	case "add":
		return output.InfoBox(
			"Add to shared VM\n" +
				"Re-provisions the shared VM with the union of existing + new packs. " +
				"All other projects that use the shared VM will see the new packs too.")
	case "use-full":
		return output.InfoBox(
			"Independent VM (just this run)\n" +
				"Switches THIS dispatch to a dedicated ephemeral VM. The project's " +
				"saved isolation setting is unchanged; future runs follow whatever " +
				"`.agent-helper.toml` says.")
	case "save-full":
		return output.InfoBox(
			"Independent VM (save the choice)\n" +
				"Updates `.agent-helper.toml` so this project always spins up a fresh " +
				"VM going forward. The shared VM is left alone.")
	case "cancel":
		return output.InfoBox(
			"Cancel\n" +
				"Aborts dispatch with a clear error. Nothing is changed.")
	}
	return ""
}
