package runner

import "os"

// collectForwardedEnv reads the named host env vars and returns the
// subset that are set, formatted as KEY=VALUE for SSHOptions.Env.
// Empty values are skipped so we don't shadow defaults inside the VM.
func collectForwardedEnv(names []string) []string {
	out := make([]string, 0, len(names))
	for _, k := range names {
		v, ok := os.LookupEnv(k)
		if !ok || v == "" {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}
