// Package args parses agent-helper-specific flags out of an argv that is
// otherwise destined for an upstream agent CLI (claude/codex/...).
//
// Recognized flags are extracted from any position before a `--` barrier;
// everything else is preserved in order. After `--`, no extraction happens
// and the remainder is strict passthrough.
package args

import (
	"fmt"
	"strings"
)

// Isolation mode for a launch.
type Isolation string

const (
	IsolationUnset  Isolation = ""
	IsolationNone   Isolation = "none"
	IsolationShared Isolation = "shared"
	IsolationFull   Isolation = "full"
)

// Parsed is the result of extracting agent-helper flags from an argv.
type Parsed struct {
	Isolated Isolation
	Local    bool
	Keep     bool
	Rm       bool
	VM       string
	NoSync   bool
	Verbose  bool

	// Args is the passthrough remainder in original order.
	// The `--` barrier itself is dropped from the output.
	Args []string
}

// Parse scans argv and extracts known agent-helper flags. Any token that is
// not a recognized agent-helper flag is appended to Parsed.Args verbatim. The
// first bare `--` token ends extraction; everything after it is appended
// verbatim (without the `--`).
func Parse(argv []string) (Parsed, error) {
	var p Parsed
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			p.Args = append(p.Args, argv[i+1:]...)
			return p, nil
		}

		// Bare boolean flags
		switch a {
		case "--local":
			p.Local = true
			continue
		case "--keep":
			p.Keep = true
			continue
		case "--rm":
			p.Rm = true
			continue
		case "--no-sync":
			p.NoSync = true
			continue
		case "--verbose":
			p.Verbose = true
			continue
		case "--isolated":
			next, ok := nextValue(argv, i)
			if !ok {
				return p, fmt.Errorf("--isolated requires a value (none|shared|full)")
			}
			p.Isolated = Isolation(next)
			i++
			continue
		case "--vm":
			next, ok := nextValue(argv, i)
			if !ok {
				return p, fmt.Errorf("--vm requires a value")
			}
			p.VM = next
			i++
			continue
		}

		// `--flag=value` forms
		if v, ok := strings.CutPrefix(a, "--isolated="); ok {
			p.Isolated = Isolation(v)
			continue
		}
		if v, ok := strings.CutPrefix(a, "--vm="); ok {
			p.VM = v
			continue
		}

		p.Args = append(p.Args, a)
	}
	return p, nil
}

// Validate enforces invariants on a parsed flag set.
func (p Parsed) Validate() error {
	switch p.Isolated {
	case IsolationUnset, IsolationNone, IsolationShared, IsolationFull:
	default:
		return fmt.Errorf("--isolated must be one of none|shared|full, got %q", p.Isolated)
	}
	if p.Local && (p.Isolated == IsolationShared || p.Isolated == IsolationFull) {
		return fmt.Errorf("--local cannot be combined with --isolated=%s (LM Studio binds to host loopback)", p.Isolated)
	}
	if p.Keep && p.Rm {
		return fmt.Errorf("--keep and --rm are mutually exclusive")
	}
	return nil
}

func nextValue(argv []string, i int) (string, bool) {
	if i+1 >= len(argv) {
		return "", false
	}
	v := argv[i+1]
	if strings.HasPrefix(v, "--") {
		return "", false
	}
	return v, true
}
