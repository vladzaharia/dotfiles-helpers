# vault-helper

> **⚠️ Work in progress (Phase 2).** The binary currently prints `vault-helper: not yet implemented` and exits. The subcommands and aliases below are the planned design — they are *not* live yet.

A HashiCorp Vault helper for SSH certificates, TOTP, env injection, and Docker registry creds.

## Planned subcommands

| Command | Purpose |
|---------|---------|
| `login` | Authenticate to Vault and cache the token |
| `ssh` | Sign an SSH key with the SSH CA backend |
| `mosh` | `ssh` + drop into a mosh session |
| `otp` | Generate a Vault-stored TOTP code |
| `token` | Print/manage the cached Vault token |
| `nv` | Run a command with secrets injected as env vars |
| `docker` | Configure Docker registry credentials from Vault |

## Planned aliases

The binary inspects `argv[0]` to dispatch to a subcommand:

| Alias | Maps to |
|-------|---------|
| `vh` | root |
| `vlogin` | `vault-helper login` |
| `vssh` | `vault-helper ssh` |
| `vmosh` | `vault-helper mosh` |
| `votp` | `vault-helper otp` |
| `vtoken` | `vault-helper token` |
| `vnv` | `vault-helper nv` |
| `vdocker` | `vault-helper docker` |

## Install

### Homebrew

```sh
brew tap vladzaharia/tap
brew install --cask vault-helper
```

The cask post-install hook creates all aliases above in `$(brew --prefix)/bin`.

### Manual

```sh
curl -fsSL https://raw.githubusercontent.com/vladzaharia/dotfiles-helpers/main/install.sh | bash -s -- vault-helper
```

### From source

```sh
go install github.com/vladzaharia/dotfiles-helpers/cmd/vault-helper@latest
```

## Status

Tracked under the [Phase 2 milestone](https://github.com/vladzaharia/dotfiles-helpers). The CLI surface and aliases are stable enough to ship the install pipeline; the implementation will land incrementally.
