package main

import (
	"github.com/vladzaharia/dotfiles-helpers/agent/cmd"
	"github.com/vladzaharia/dotfiles-helpers/internal/alias"
)

func main() {
	alias.RewriteArgs("agent-helper", map[string]string{
		"ag": "",
	})
	cmd.Execute()
}
