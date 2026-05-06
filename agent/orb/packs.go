package orb

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed packs/*.toml
var packsFS embed.FS

// Category groups packs by purpose; used for help/list output. Not a
// behavior knob — both "agent" and "language" packs install identically.
type Category string

const (
	CategoryCore     Category = "core"
	CategoryLanguage Category = "language"
	CategoryAgent    Category = "agent"
	CategoryTools    Category = "tools"
	CategoryMeta     Category = "meta"
)

// Pack is one composable bootstrap unit, loaded from packs/<name>.toml.
type Pack struct {
	Name        string   `toml:"name"`
	Category    Category `toml:"category"`
	Description string   `toml:"description"`
	Deps        []string `toml:"deps"`
	Packages    []string `toml:"packages"` // apt packages
	Runcmd      []string `toml:"runcmd"`   // shell lines (single line each)
	WriteFiles  []struct {
		Path        string `toml:"path"`
		Content     string `toml:"content"`
		Permissions string `toml:"permissions"`
	} `toml:"write_files"`
}

// LoadPacks parses every embedded packs/*.toml. The map is keyed by pack name.
func LoadPacks() (map[string]*Pack, error) {
	entries, err := packsFS.ReadDir("packs")
	if err != nil {
		return nil, err
	}
	out := map[string]*Pack{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
			continue
		}
		raw, err := packsFS.ReadFile("packs/" + e.Name())
		if err != nil {
			return nil, err
		}
		var p Pack
		if _, err := toml.Decode(string(raw), &p); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if p.Name == "" {
			p.Name = strings.TrimSuffix(e.Name(), ".toml")
		}
		out[p.Name] = &p
	}
	return out, nil
}

// Resolve returns the topologically-ordered transitive closure of `wanted`
// through Deps. The implicit `base` pack is always first. A cycle is an error.
func Resolve(wanted []string, all map[string]*Pack) ([]*Pack, error) {
	seen := map[string]bool{}
	temp := map[string]bool{}
	var order []*Pack

	var visit func(name string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		if temp[name] {
			return fmt.Errorf("pack dependency cycle through %q", name)
		}
		p, ok := all[name]
		if !ok {
			return fmt.Errorf("unknown pack %q", name)
		}
		temp[name] = true
		for _, d := range p.Deps {
			if err := visit(d); err != nil {
				return err
			}
		}
		temp[name] = false
		seen[name] = true
		order = append(order, p)
		return nil
	}

	// Always pull base first.
	if _, hasBase := all["base"]; hasBase {
		if err := visit("base"); err != nil {
			return nil, err
		}
	}
	for _, w := range wanted {
		if err := visit(w); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// Compose assembles a cloud-init YAML from an ordered pack list and stamps
// a content-hash version into /etc/agent-helper/bootstrap.version. The
// returned runcmd slice is the same shell statements emitted in the YAML,
// useful for live reseeding (Phase 10).
func Compose(packs []*Pack, agent string) (yaml []byte, version string, runcmd []string) {
	// Hash inputs for deterministic version.
	h := sha256.New()
	var pkgs []string
	pkgSeen := map[string]bool{}
	var runcmds []string
	type wf struct{ Path, Content, Permissions string }
	var files []wf

	for _, p := range packs {
		fmt.Fprintln(h, p.Name)
		for _, k := range p.Packages {
			if !pkgSeen[k] {
				pkgs = append(pkgs, k)
				pkgSeen[k] = true
			}
		}
		runcmds = append(runcmds, p.Runcmd...)
		for _, f := range p.WriteFiles {
			files = append(files, wf{f.Path, f.Content, f.Permissions})
		}
		h.Write([]byte(strings.Join(p.Packages, ",")))
		h.Write([]byte(strings.Join(p.Runcmd, "\n")))
	}
	version = hex.EncodeToString(h.Sum(nil))[:12]

	// Marker files for agent-helper.
	files = append(files, wf{"/etc/agent-helper/bootstrap.version", version, "0644"})
	files = append(files, wf{"/etc/agent-helper/agent", agent, "0644"})
	files = append(files, wf{
		Path: "/etc/agent-helper/packs",
		Content: strings.Join(packNames(packs), "\n") + "\n",
		Permissions: "0644",
	})

	// Render YAML by hand — predictable, no extra dep.
	var b strings.Builder
	b.WriteString("#cloud-config\n")
	b.WriteString("package_update: true\n")
	if len(pkgs) > 0 {
		b.WriteString("packages:\n")
		sort.Strings(pkgs)
		for _, p := range pkgs {
			b.WriteString("  - ")
			b.WriteString(p)
			b.WriteByte('\n')
		}
	}
	if len(files) > 0 {
		b.WriteString("write_files:\n")
		for _, f := range files {
			b.WriteString("  - path: ")
			b.WriteString(yamlString(f.Path))
			b.WriteByte('\n')
			b.WriteString("    content: ")
			if strings.Contains(f.Content, "\n") {
				b.WriteString("|\n")
				for _, line := range strings.Split(strings.TrimRight(f.Content, "\n"), "\n") {
					b.WriteString("      ")
					b.WriteString(line)
					b.WriteByte('\n')
				}
			} else {
				b.WriteString(yamlString(f.Content))
				b.WriteByte('\n')
			}
			perm := f.Permissions
			if perm == "" {
				perm = "0644"
			}
			b.WriteString("    permissions: ")
			b.WriteString(yamlString(perm))
			b.WriteByte('\n')
		}
	}
	b.WriteString("runcmd:\n")
	b.WriteString("  - mkdir -p /etc/agent-helper /opt/pipx /usr/local/share/agent-helper\n")
	for _, line := range runcmds {
		b.WriteString("  - ")
		b.WriteString(yamlString(line))
		b.WriteByte('\n')
	}
	b.WriteString("final_message: \"agent-helper bootstrap complete after $UPTIME seconds\"\n")
	return []byte(b.String()), version, runcmds
}

func yamlString(s string) string {
	if s == "" {
		return `""`
	}
	// always emit double-quoted to be safe with shell pipelines / colons
	esc := strings.ReplaceAll(s, `\`, `\\`)
	esc = strings.ReplaceAll(esc, `"`, `\"`)
	return `"` + esc + `"`
}

func packNames(packs []*Pack) []string {
	out := make([]string, 0, len(packs))
	for _, p := range packs {
		out = append(out, p.Name)
	}
	return out
}
