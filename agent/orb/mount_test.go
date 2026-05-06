package orb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSharedRoots(t *testing.T) {
	home, _ := os.UserHomeDir()
	specs := SharedRoots([]string{"~/Repos", "~/Repos", "/tmp/agents"})
	if len(specs) != 2 {
		t.Fatalf("expected 2 unique specs, got %d (%v)", len(specs), specs)
	}
	if specs[0].Source != filepath.Join(home, "Repos") || specs[0].Dest != "/host/Repos" {
		t.Errorf("unexpected first spec: %+v", specs[0])
	}
	if specs[1].Source != "/tmp/agents" || specs[1].Dest != "/host/agents" {
		t.Errorf("unexpected second spec: %+v", specs[1])
	}
}

func TestResolveCWD(t *testing.T) {
	specs := []MountSpec{{Source: "/Users/vlad/Repos", Dest: "/host/Repos"}}
	cases := []struct {
		cwd, want string
		ok        bool
	}{
		{"/Users/vlad/Repos/foo", "/host/Repos/foo", true},
		{"/Users/vlad/Repos", "/host/Repos", true},
		{"/Users/vlad/Repos/a/b/c", "/host/Repos/a/b/c", true},
		{"/Users/vlad/Documents", "", false},
		{"/tmp/scratch", "", false},
	}
	for _, tc := range cases {
		got, ok := ResolveCWD(tc.cwd, specs)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ResolveCWD(%q) = (%q,%v), want (%q,%v)", tc.cwd, got, ok, tc.want, tc.ok)
		}
	}
}

func TestDedicatedMount(t *testing.T) {
	tmp, _ := os.MkdirTemp("", "agent-test-*")
	defer os.RemoveAll(tmp)

	spec, vmPath, err := DedicatedMount(tmp)
	if err != nil {
		t.Fatal(err)
	}
	want := "/work/" + filepath.Base(tmp)
	if vmPath != want || spec.Dest != want {
		t.Errorf("got vm path %q, want %q", vmPath, want)
	}
	if spec.Source != tmp {
		t.Errorf("source = %q, want %q", spec.Source, tmp)
	}
}

func TestStateMountsSkipsMissingPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USER", "vlad")
	// One that exists, one that doesn't.
	if err := os.MkdirAll(filepath.Join(tmp, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	specs := StateMounts([]string{"~/.claude", "~/.does-not-exist", "/abs/path"})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d (%v)", len(specs), specs)
	}
	if specs[0].Source != filepath.Join(tmp, ".claude") {
		t.Errorf("source = %q, want %q", specs[0].Source, filepath.Join(tmp, ".claude"))
	}
	if specs[0].Dest != "/home/vlad/.claude" {
		t.Errorf("dest = %q, want /home/vlad/.claude", specs[0].Dest)
	}
}

func TestStateMountsTranslatesFilesAndDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USER", "vlad")
	// Both a directory and a single file should pass through.
	if err := os.MkdirAll(filepath.Join(tmp, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".claude.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	specs := StateMounts([]string{"~/.claude", "~/.claude.json"})
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
}

func TestMountSpecString(t *testing.T) {
	cases := []struct {
		spec MountSpec
		want string
	}{
		{MountSpec{"/a", "/a"}, "/a"},
		{MountSpec{"/a", ""}, "/a"},
		{MountSpec{"/a", "/b"}, "/a:/b"},
	}
	for _, tc := range cases {
		if got := tc.spec.String(); got != tc.want {
			t.Errorf("(%+v).String() = %q, want %q", tc.spec, got, tc.want)
		}
	}
}
