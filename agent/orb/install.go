package orb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// EnsureInstalled installs OrbStack on macOS when missing. On Linux it
// returns an error (OrbStack is macOS-only as of writing). When OrbStack
// is already installed it's a no-op.
//
// On macOS the strategy is:
//  1. If `brew` is on PATH → `brew install --cask orbstack`.
//  2. Otherwise → download the official .dmg, attach, copy OrbStack.app
//     into /Applications, detach, and clean up.
//
// Progress is streamed to w (typically os.Stderr). Cancellation via ctx
// is honored at every subprocess boundary.
func EnsureInstalled(ctx context.Context, w io.Writer) error {
	if IsInstalled() {
		return nil
	}
	if runtime.GOOS != "darwin" {
		return errors.New("OrbStack auto-install is only supported on macOS")
	}
	if _, err := exec.LookPath("brew"); err == nil {
		return installViaBrew(ctx, w)
	}
	return installViaDMG(ctx, w)
}

func installViaBrew(ctx context.Context, w io.Writer) error {
	fmt.Fprintln(w, "  Installing OrbStack via Homebrew...")
	cmd := exec.CommandContext(ctx, "brew", "install", "--cask", "orbstack")
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew install --cask orbstack: %w", err)
	}
	return nil
}

func installViaDMG(ctx context.Context, w io.Writer) error {
	arch := runtime.GOARCH
	if arch != "arm64" && arch != "amd64" {
		return fmt.Errorf("unsupported arch for OrbStack DMG install: %s", arch)
	}
	url := "https://orbstack.dev/download/stable/latest/" + arch

	tmpDir, err := os.MkdirTemp("", "agent-helper-orbstack-*")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	dmg := filepath.Join(tmpDir, "OrbStack.dmg")

	fmt.Fprintln(w, "  Downloading OrbStack from", url)
	curl := exec.CommandContext(ctx, "curl", "-fL", "--progress-bar", "-o", dmg, url)
	curl.Stdout = w
	curl.Stderr = w
	if err := curl.Run(); err != nil {
		return fmt.Errorf("download OrbStack.dmg: %w", err)
	}

	fmt.Fprintln(w, "  Mounting disk image...")
	attach := exec.CommandContext(ctx, "hdiutil", "attach", "-nobrowse", "-readonly", dmg)
	attachOut, err := attach.Output()
	if err != nil {
		return fmt.Errorf("hdiutil attach: %w", err)
	}
	mount := parseHdiutilMount(string(attachOut))
	if mount == "" {
		return fmt.Errorf("could not determine OrbStack mount point from hdiutil output")
	}
	defer func() {
		detach := exec.Command("hdiutil", "detach", "-quiet", mount)
		detach.Stdout = w
		detach.Stderr = w
		_ = detach.Run()
	}()

	src := filepath.Join(mount, "OrbStack.app")
	dst := "/Applications/OrbStack.app"
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("OrbStack.app not found at %s: %w", src, err)
	}

	fmt.Fprintln(w, "  Copying OrbStack.app to /Applications...")
	if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing %s: %w", dst, err)
	}
	cp := exec.CommandContext(ctx, "cp", "-R", src, dst)
	cp.Stdout = w
	cp.Stderr = w
	if err := cp.Run(); err != nil {
		return fmt.Errorf("copy OrbStack.app: %w", err)
	}
	return nil
}

// parseHdiutilMount extracts the mount-point column from `hdiutil attach`
// plain-text output. Each line is "<device>\t<content>\t<mount>"; the
// OrbStack volume is the line whose mount-point is non-empty.
func parseHdiutilMount(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		mp := strings.TrimSpace(fields[len(fields)-1])
		if strings.HasPrefix(mp, "/Volumes/") {
			return mp
		}
	}
	return ""
}

// WaitForDaemon polls IsRunning until it succeeds or `timeout` elapses.
// On macOS it nudges OrbStack via `open -a OrbStack` once at the start to
// trigger the helper daemon.
func WaitForDaemon(ctx context.Context, timeout time.Duration) error {
	if !IsInstalled() {
		return errors.New("orb CLI not on PATH")
	}
	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", "-a", "OrbStack").Run()
	}
	deadline := time.Now().Add(timeout)
	for {
		if IsRunning() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("OrbStack daemon did not start within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
