# dotfiles-helpers

A trio of Go CLIs I use across my machines:

| Tool | Purpose | Aliases |
|------|---------|---------|
| [**agent-helper**](cmd/agent-helper/README.md) | Unified dispatcher for AI coding agents (Claude, Codex, etc.) | `ag` |
| [**vault-helper**](cmd/vault-helper/README.md) | HashiCorp Vault helper — SSH certs, TOTP, env injection, Docker creds *(WIP)* | `vh`, `vlogin`, `vssh`, `vmosh`, `votp`, `vtoken`, `vnv`, `vdocker` |
| [**sops-helper**](cmd/sops-helper/README.md) | SOPS encrypt/decrypt with glob support *(WIP)* | `crypto` |

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

## Development

A `Makefile` at the root and per-helper Makefiles under `cmd/<helper>/` cover the common loops:

```sh
make help                          # list all root targets
make test                          # go test ./...
make build                         # go build ./...
make vet tidy lint                 # go vet, go mod tidy, golangci-lint
make clean                         # rm dist/ and per-helper bin/

cd cmd/agent-helper && make run ARGS="--help"   # build & run one helper
cd cmd/agent-helper && make install             # go install to $GOBIN
```

## Releasing (maintainer)

All three helpers ship together under one `vX.Y.Z` tag — `make release` is the one-command flow:

```sh
make release          # patch bump (0.6.3 → 0.6.4)
make release-minor    # minor bump (0.6.3 → 0.7.0)
make release-major    # major bump
make release-dry      # local goreleaser snapshot — no tag, no push
```

The script (`scripts/release.sh`) preflights a clean tree, the `main` branch, sync with origin, and `go test`, then shows a changelog preview and asks before tagging. CI (`.github/workflows/release.yml`) takes over on push: goreleaser builds artifacts for all three helpers, publishes the GitHub release, and bumps the brew tap. After CI completes:

```sh
brew upgrade --cask agent-helper vault-helper sops-helper
```
