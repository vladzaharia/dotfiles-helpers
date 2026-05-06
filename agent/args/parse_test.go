package args

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want Parsed
	}{
		{
			name: "no flags, all passthrough",
			argv: []string{"--resume", "abc"},
			want: Parsed{Args: []string{"--resume", "abc"}},
		},
		{
			name: "extracts isolated and keeps passthrough order",
			argv: []string{"--resume", "--isolated", "full", "--keep", "abc"},
			want: Parsed{Isolated: IsolationFull, Keep: true, Args: []string{"--resume", "abc"}},
		},
		{
			name: "isolated equals form",
			argv: []string{"--isolated=shared", "--", "--resume"},
			want: Parsed{Isolated: IsolationShared, Args: []string{"--resume"}},
		},
		{
			name: "double dash forces passthrough of would-be ah flags",
			argv: []string{"--", "--keep", "--rm"},
			want: Parsed{Args: []string{"--keep", "--rm"}},
		},
		{
			name: "vm name with space form",
			argv: []string{"--vm", "myproj", "-c"},
			want: Parsed{VM: "myproj", Args: []string{"-c"}},
		},
		{
			name: "local + bool flags",
			argv: []string{"--local", "--no-sync", "--verbose"},
			want: Parsed{Local: true, NoSync: true, Verbose: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.argv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []string{"--isolated"}
	for _, in := range cases {
		if _, err := Parse([]string{in}); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		p       Parsed
		wantErr bool
	}{
		{"empty ok", Parsed{}, false},
		{"none ok", Parsed{Isolated: IsolationNone}, false},
		{"local + shared fails", Parsed{Local: true, Isolated: IsolationShared}, true},
		{"local + none ok", Parsed{Local: true, Isolated: IsolationNone}, false},
		{"keep + rm fails", Parsed{Keep: true, Rm: true}, true},
		{"bad isolated", Parsed{Isolated: "weird"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.p.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
