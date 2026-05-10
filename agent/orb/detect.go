package orb

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DetectionEvidence records why a pack was suggested. The wizard uses
// it to show "package.json found" hints next to detected packs.
type DetectionEvidence struct {
	Pack  string
	Files []string // file/dir basenames that matched
	Dir   string   // where they were found, relative to walk root
}

// IsolationDecision is the auto-suggested isolation mode for a project
// + the human-readable reason shown in the wizard infobox.
type IsolationDecision struct {
	Mode   string // "none" | "shared" | "full"
	Reason string
}

// projectSignals maps pack names to the file/dir basenames that
// indicate the project uses that toolchain. Order matters only for
// ergonomics; detection runs all checks per directory.
func projectSignals() map[string][]string {
	return map[string][]string{
		"node":   {"package.json", "pnpm-lock.yaml", "yarn.lock"},
		"docker": {"Dockerfile", "docker-compose.yaml", "docker-compose.yml", "compose.yaml", "compose.yml"},
		"go":     {"go.mod"},
		"rust":   {"Cargo.toml"},
		"python": {"pyproject.toml", "requirements.txt", "setup.py", "Pipfile"},
		"ruby":   {"Gemfile"},
		"java":   {"pom.xml", "build.gradle", "build.gradle.kts"},
		"dotnet": {"*.csproj", "*.sln", "*.fsproj"},
		"mise":   {"mise.toml", ".tool-versions", ".mise.toml"},
	}
}

// deviceDevSignals lists file/dir markers that indicate the project
// requires direct host access (Xcode toolchain, Android SDK, native
// device APIs). When any are present, isolation defaults to "none".
func deviceDevSignals() []string {
	return []string{
		"*.xcodeproj",   // iOS / macOS
		"*.xcworkspace", // iOS / macOS
		"Podfile",       // CocoaPods
		"Package.swift", // SwiftPM (often for Apple platforms)
	}
}

// skipDirs are directories the walker never recurses into. These are
// almost always either build artifacts (which we don't care about) or
// huge enough to make the walk slow.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"vendor":       true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	".next":        true,
	".nuxt":        true,
	".cache":       true,
}

// DetectProjectPacks returns the union of language/toolchain packs
// inferred from the project tree, plus evidence for each match.
//
// Walk policy (matches the plan):
//   - CWD itself + 3 levels of subdirectories beneath it
//   - The 3 immediate parent directories of CWD, each inspected as a
//     single directory (not recursed)
//   - Stop at $HOME — never traverse above it
//   - Skip well-known dependency/build dirs (skipDirs)
func DetectProjectPacks(cwd string) ([]string, []DetectionEvidence) {
	signals := projectSignals()
	hits := map[string]*DetectionEvidence{}
	addHit := func(pack, file, dir string) {
		if hits[pack] == nil {
			hits[pack] = &DetectionEvidence{Pack: pack, Dir: dir}
		}
		hits[pack].Files = append(hits[pack].Files, file)
	}

	home, _ := os.UserHomeDir()
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, nil
	}

	// Walk CWD + 3 subdir levels deep.
	maxDepth := 3
	_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(abs, path)
			depth := strings.Count(rel, string(os.PathSeparator))
			if path != abs && (skipDirs[d.Name()] || depth >= maxDepth) {
				return fs.SkipDir
			}
			return nil
		}
		// File: check against all signals.
		base := d.Name()
		relDir, _ := filepath.Rel(abs, filepath.Dir(path))
		if relDir == "." {
			relDir = ""
		}
		for pack, patterns := range signals {
			for _, pat := range patterns {
				if matchSignal(base, pat) {
					addHit(pack, base, relDir)
					break
				}
			}
		}
		return nil
	})

	// Inspect each parent dir (3 levels up) as a single directory.
	parent := filepath.Dir(abs)
	for i := 0; i < 3 && parent != "" && parent != "/" && (home == "" || withinHome(parent, home)); i++ {
		entries, _ := os.ReadDir(parent)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			base := e.Name()
			for pack, patterns := range signals {
				for _, pat := range patterns {
					if matchSignal(base, pat) {
						addHit(pack, base, parent)
						break
					}
				}
			}
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}

	out := make([]string, 0, len(hits))
	evidence := make([]DetectionEvidence, 0, len(hits))
	for _, ev := range hits {
		out = append(out, ev.Pack)
		evidence = append(evidence, *ev)
	}
	return out, evidence
}

// DetectProjectIsolation picks the auto-suggested isolation mode for a
// project. Three rules in priority order:
//
//  1. Device development (iOS, macOS native, Android) → "none"
//  2. CWD inside the shared-VM mount territory → "shared"
//  3. Otherwise → "full"
//
// sharedMounts is the list of host paths the configured shared VM
// mounts (typically cfg.Orb.MountRoots). When the shared VM doesn't
// exist yet, pass nil — rule 2 won't fire and we'll default to "full".
func DetectProjectIsolation(cwd string, sharedExists bool, sharedMounts []string) IsolationDecision {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return IsolationDecision{Mode: "full", Reason: "couldn't resolve CWD; defaulting to ephemeral VM"}
	}

	if sig := detectDeviceDev(abs); sig != "" {
		return IsolationDecision{
			Mode:   "none",
			Reason: "Device-development project detected (" + sig + "). VMs can't run platform toolchains like Xcode or Android SDK signing.",
		}
	}

	if sharedExists {
		for _, root := range sharedMounts {
			expanded := expandHome(root)
			if expanded != "" && (abs == expanded || strings.HasPrefix(abs, expanded+string(os.PathSeparator))) {
				return IsolationDecision{
					Mode:   "shared",
					Reason: "Directory is inside the shared VM's mount territory (" + root + "). Reusing the warm VM avoids a fresh-startup cost per task.",
				}
			}
		}
	}

	return IsolationDecision{
		Mode:   "full",
		Reason: "No special signals detected. Dedicated ephemeral VM per task is the safest default.",
	}
}

// detectDeviceDev returns the signal that triggered (e.g. "*.xcodeproj")
// or "" when no device-dev markers found in CWD or its first level of
// subdirectories. We check the project root and one level down because
// "ios/" or "android/" sub-projects are common.
func detectDeviceDev(abs string) string {
	check := func(dir string) string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			name := e.Name()
			for _, sig := range deviceDevSignals() {
				if matchSignal(name, sig) {
					return sig
				}
			}
			// android/ + build.gradle is the React Native / Flutter
			// signal. We don't currently descend, so we approximate with
			// "android/" presence at depth 0 or 1.
			if e.IsDir() && (name == "android" || name == "ios") {
				if hasAnyFile(filepath.Join(dir, name), []string{"build.gradle", "build.gradle.kts", "app/build.gradle"}) ||
					hasAnyMatch(filepath.Join(dir, name), []string{"*.xcodeproj", "*.xcworkspace"}) {
					return name + "/"
				}
			}
		}
		return ""
	}
	if sig := check(abs); sig != "" {
		return sig
	}
	// Also check direct subdirectories — monorepos often nest the
	// platform project a level down.
	entries, _ := os.ReadDir(abs)
	for _, e := range entries {
		if !e.IsDir() || skipDirs[e.Name()] {
			continue
		}
		if sig := check(filepath.Join(abs, e.Name())); sig != "" {
			return sig
		}
	}
	return ""
}

// matchSignal returns true when name matches the signal pattern. The
// pattern is either a literal filename or a "*.ext" glob.
func matchSignal(name, pattern string) bool {
	if !strings.ContainsAny(pattern, "*?[") {
		return name == pattern
	}
	matched, _ := filepath.Match(pattern, name)
	return matched
}

func hasAnyFile(dir string, names []string) bool {
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

func hasAnyMatch(dir string, patterns []string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		for _, p := range patterns {
			if matchSignal(e.Name(), p) {
				return true
			}
		}
	}
	return false
}

func withinHome(path, home string) bool {
	return path == home || strings.HasPrefix(path, home+string(os.PathSeparator))
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
