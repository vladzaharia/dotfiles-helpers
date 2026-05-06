package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// resolveProjectPacks merges the configured default packs with any
// `.agentpacks` file found in (or above) the current working directory.
//
// .agentpacks format (gitignore-ish):
//
//	# comment
//	rust          # add this pack
//	+ dotnet      # explicit add
//	- python      # remove from defaults
func resolveProjectPacks(defaults []string, cwd string) []string {
	set := map[string]bool{}
	for _, p := range defaults {
		set[p] = true
	}
	if path := findAgentPacks(cwd); path != "" {
		f, err := os.Open(path)
		if err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if c := strings.Index(line, "#"); c >= 0 {
					line = strings.TrimSpace(line[:c])
				}
				switch {
				case strings.HasPrefix(line, "+"):
					set[strings.TrimSpace(line[1:])] = true
				case strings.HasPrefix(line, "-"):
					delete(set, strings.TrimSpace(line[1:]))
				default:
					set[line] = true
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

// findAgentPacks walks up from cwd to find a `.agentpacks` file.
// Returns "" if none.
func findAgentPacks(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(abs, ".agentpacks")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}
