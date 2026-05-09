# dotfiles-helpers

A trio of Go CLIs I use across my machines:

| Tool | Purpose | Aliases |
|------|---------|---------|
| **agent-helper** | Unified dispatcher for AI coding agents (Claude, Codex, etc.) | `ag` |
| **vault-helper** | HashiCorp Vault helper — SSH certs, TOTP, env injection, Docker creds | `vh`, `vlogin`, `vssh`, `vmosh`, `votp`, `vtoken`, `vnv`, `vdocker` |
| **sops-helper** | SOPS encrypt/decrypt with glob support | `crypto` |

Releases are built by [goreleaser](https://goreleaser.com) for macOS and Linux on `amd64` and `arm64`.

## Install

### Homebrew (macOS / Linux)

Each helper is published as a cask in [`vladzaharia/homebrew-tap`](https://github.com/vladzaharia/homebrew-tap):

```sh
brew tap vladzaharia/tap
brew install --cask agent-helper vault-helper sops-helper
```

The cask post-install hooks strip the quarantine attribute and create the alias symlinks listed above in `$(brew --prefix)/bin`.

To upgrade:

```sh
brew upgrade --cask agent-helper vault-helper sops-helper
```

### Manual (curl)

The `install.sh` script downloads the latest release tarball from GitHub and drops the binary plus its aliases into `$INSTALL_DIR` (default `~/.local/bin`):

```sh
# Install everything
curl -fsSL https://raw.githubusercontent.com/vladzaharia/dotfiles-helpers/main/install.sh | bash

# Install one tool
curl -fsSL https://raw.githubusercontent.com/vladzaharia/dotfiles-helpers/main/install.sh | bash -s -- vault-helper

# Custom install dir
curl -fsSL https://raw.githubusercontent.com/vladzaharia/dotfiles-helpers/main/install.sh | INSTALL_DIR=/usr/local/bin bash
```

Make sure `$INSTALL_DIR` is on your `$PATH`.

### Linux package managers (deb / rpm)

`.deb` and `.rpm` packages are attached to each [GitHub release](https://github.com/vladzaharia/dotfiles-helpers/releases):

```sh
# Debian / Ubuntu
curl -fsSLO https://github.com/vladzaharia/dotfiles-helpers/releases/latest/download/agent-helper_<version>_linux_amd64.deb
sudo dpkg -i agent-helper_<version>_linux_amd64.deb

# Fedora / RHEL
sudo rpm -i https://github.com/vladzaharia/dotfiles-helpers/releases/latest/download/agent-helper_<version>_linux_amd64.rpm
```

### From source

```sh
git clone https://github.com/vladzaharia/dotfiles-helpers
cd dotfiles-helpers
go install ./cmd/agent-helper ./cmd/vault-helper ./cmd/sops-helper
```

## Usage

Each tool ships with its own help:

```sh
ag --help
vh --help
crypto --help
```
