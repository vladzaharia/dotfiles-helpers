package alias

import (
	"os"
	"path/filepath"
)

// RewriteArgs checks if the binary was invoked via a symlink alias
// and rewrites os.Args to inject the corresponding subcommand.
// This enables busybox-style invocation: "vssh host" → "vault-helper ssh host".
func RewriteArgs(binaryName string, aliasMap map[string]string) {
	base := filepath.Base(os.Args[0])
	if sub, ok := aliasMap[base]; ok {
		os.Args = append([]string{binaryName, sub}, os.Args[1:]...)
	}
}
