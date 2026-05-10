# sops-helper

> **⚠️ Work in progress (Phase 3).** The binary currently prints `sops-helper: not yet implemented` and exits. The design below is the planned surface.

A [`sops`](https://github.com/getsops/sops) wrapper for encrypting/decrypting secrets across globbed file sets — useful for dotfiles repos that mix plaintext and encrypted YAML/JSON/env files.

## Planned subcommands

| Command | Purpose |
|---------|---------|
| `encrypt <glob>` | Encrypt one or more files in place |
| `decrypt <glob>` | Decrypt one or more files in place |
| `edit <file>` | Open an encrypted file in `$EDITOR` |
| `keys` | Manage age recipients / KMS bindings |

## Planned aliases

| Alias | Maps to |
|-------|---------|
| `crypto` | root |

## Install

### Homebrew

```sh
brew tap vladzaharia/tap
brew install --cask sops-helper
```

### Manual

```sh
curl -fsSL https://raw.githubusercontent.com/vladzaharia/dotfiles-helpers/main/install.sh | bash -s -- sops-helper
```

### From source

```sh
go install github.com/vladzaharia/dotfiles-helpers/cmd/sops-helper@latest
```

## Status

Tracked under the [Phase 3 milestone](https://github.com/vladzaharia/dotfiles-helpers). Depends on a working installation of `sops` and at least one configured age key or KMS binding.
