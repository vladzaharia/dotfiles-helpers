# agent-helper

Unified dispatcher for AI coding agents (Claude Code, Codex CLI, …) with first-class OrbStack VM isolation and LM Studio integration for local models.

## What it does

- **Dispatch**: `ag` (or `agent-helper`) routes to the configured default agent. `ag claude`, `ag codex`, `ag local` invoke specific providers.
- **Isolation modes** (macOS only, requires OrbStack):
  - `none` — run on host
  - `shared` — long-lived per-provider VM, mounts your repo paths live
  - `full` — dedicated ephemeral VM per project (default)
- **Local models**: `agent-helper local` (or `ag --local`) routes Claude Code at LM Studio, picking models tier-mapped to Haiku / Sonnet / Opus.
- **Cred forwarding**: forwards `CLAUDE_CODE_OAUTH_TOKEN` / `ANTHROPIC_*` and `~/.claude/.credentials.json` into VM sessions.

## Install

### Homebrew (recommended)

```sh
brew tap vladzaharia/tap
brew install --cask agent-helper
```

The cask creates the `ag` symlink in `$(brew --prefix)/bin` and strips the macOS quarantine attribute.

### Manual

```sh
curl -fsSL https://raw.githubusercontent.com/vladzaharia/dotfiles-helpers/main/install.sh | bash -s -- agent-helper
```

Installs to `~/.local/bin` by default. Override with `INSTALL_DIR`.

### From source

```sh
go install github.com/vladzaharia/dotfiles-helpers/cmd/agent-helper@latest
```

## First run

Running `ag` (or `agent-helper`) with no config triggers the interactive setup wizard. It will ask:

1. **Default agent** — claude or codex.
2. **Default isolation** *(macOS only)* — none / shared / full.
3. **OrbStack** *(if isolation ≠ none)* — installs via Homebrew if available, otherwise downloads the official `.dmg`.
4. **Default packs + mount roots** — pre-installed cloud-init packs (e.g. `nodejs`, `python`, `rust`) and host directories to live-mount into VMs.
5. **Provider binary overrides** — explicit paths for `claude` / `codex` (blank = use `$PATH`).
6. **LM Studio** — URL + per-tier (Haiku / Sonnet / Opus) model selection. Models are auto-suggested by parameter count and `reasoning` capability.
7. **Claude credentials** — `claude setup-token` for a long-lived OAuth token, or sync from macOS Keychain.

Re-run any time with `agent-helper setup`. Use `--non-interactive` (also auto-detected when stdin isn't a TTY) to write defaults and exit.

## Subcommands

| Command | Purpose |
|---------|---------|
| `setup` | Interactive setup wizard |
| `status` | Show provider detection + auth status |
| `claude [flags]` | Launch Claude Code |
| `codex [flags]` | Launch Codex CLI |
| `local [flags]` | Launch Claude Code against LM Studio (`--local`) |
| `vm` | Manage agent-helper OrbStack VMs (`list`, `shell`, `reset`, `prune`) |
| `doctor` | Diagnose environment + prune idle VMs |
| `packs` | List/inspect cloud-init packs |
| `auth claude` | Inspect or export Claude credentials for VM use |
| `config` | Inspect/edit TOML config (`show`, `path`, `edit`) |

## Aliases

`agent-helper` looks at its `argv[0]`. Symlinking it as any of these short names invokes the matching subcommand:

- `ag` → root (runs the default agent)
- `claude` → `agent-helper claude`
- `codex` → `agent-helper codex`

The Homebrew cask creates `ag` automatically.

## Config

| File | Purpose |
|------|---------|
| `~/.config/agent-helper/config.toml` | Global defaults (TOML) |
| `~/.config/agent-helper/secrets.toml` | OAuth tokens (mode 0600) |
| `.agent-helper.toml` | Per-project overrides (walks up from CWD) |

Inspect with `agent-helper config show`; open in `$EDITOR` with `agent-helper config edit`.

## Environment

- `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_API_KEY`, `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` — forwarded to VM sessions when set.
- `XDG_CONFIG_HOME` — respected for config path resolution.

## Examples

```sh
ag                         # default agent, default isolation
ag claude --isolated none  # one-off on host
ag local                   # Claude Code → LM Studio
agent-helper vm list       # see provisioned OrbStack VMs
agent-helper doctor        # health check
```
