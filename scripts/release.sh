#!/bin/bash
# Cut a release: preflight → version bump → changelog preview → tag → push.
# CI (.github/workflows/release.yml) takes over from there: goreleaser builds
# artifacts, publishes the GitHub release, and bumps the brew tap.

set -euo pipefail

REPO_OWNER="vladzaharia"
REPO_NAME="dotfiles-helpers"
MAIN_BRANCH="main"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

# ── Flags / args ─────────────────────────────────────────────────────────
DRY_RUN=0
SKIP_TESTS=0
FORCE_BRANCH=0
ASSUME_YES=0
TARGET=""

usage() {
    cat <<EOF
Usage: scripts/release.sh [bump|version] [flags]

Bumps:
  patch (default), minor, major, or explicit vX.Y.Z / X.Y.Z

Flags:
  --dry-run         Run goreleaser snapshot locally; no tag, no push
  --yes, -y         Skip confirmation prompt
  --skip-tests      Skip 'go test ./...' preflight
  --force-branch    Allow releasing from a non-${MAIN_BRANCH} branch
  --help, -h        Show this help

Examples:
  scripts/release.sh                  # patch bump (default)
  scripts/release.sh minor            # minor bump
  scripts/release.sh v0.7.0           # explicit version
  scripts/release.sh --dry-run        # local snapshot only
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help) usage; exit 0 ;;
        --dry-run) DRY_RUN=1 ;;
        -y|--yes) ASSUME_YES=1 ;;
        --skip-tests) SKIP_TESTS=1 ;;
        --force-branch) FORCE_BRANCH=1 ;;
        -*) echo "error: unknown flag '$1'" >&2; usage >&2; exit 1 ;;
        *)
            if [[ -z "$TARGET" ]]; then
                TARGET="$1"
            else
                echo "error: unexpected positional arg '$1'" >&2
                exit 1
            fi
            ;;
    esac
    shift
done

TARGET="${TARGET:-patch}"

# ── Preflight ────────────────────────────────────────────────────────────
require_clean_repo() {
    if [[ -n "$(git status --porcelain)" ]]; then
        echo "error: uncommitted changes in working tree" >&2
        git status --short >&2
        exit 1
    fi
}

require_branch_main() {
    local branch
    branch="$(git rev-parse --abbrev-ref HEAD)"
    if [[ "$branch" != "$MAIN_BRANCH" && $FORCE_BRANCH -eq 0 ]]; then
        echo "error: not on '$MAIN_BRANCH' (currently on '$branch')" >&2
        echo "       use --force-branch to override" >&2
        exit 1
    fi
}

require_synced() {
    git fetch --tags origin "$MAIN_BRANCH" >/dev/null 2>&1 || {
        echo "error: git fetch failed" >&2
        exit 1
    }
    local local_sha remote_sha
    local_sha="$(git rev-parse HEAD)"
    remote_sha="$(git rev-parse "origin/$MAIN_BRANCH")"
    if [[ "$local_sha" != "$remote_sha" ]]; then
        echo "error: local '$MAIN_BRANCH' is not in sync with origin/$MAIN_BRANCH" >&2
        echo "       local:  $local_sha" >&2
        echo "       remote: $remote_sha" >&2
        exit 1
    fi
}

require_tests_pass() {
    if [[ $SKIP_TESTS -eq 1 ]]; then
        echo "  (skipping 'go test ./...' per --skip-tests)"
        return
    fi
    echo "  Running tests..."
    go test ./... >/dev/null
}

require_goreleaser() {
    command -v goreleaser >/dev/null || {
        echo "error: goreleaser not installed" >&2
        echo "       brew install goreleaser  (or see https://goreleaser.com/install)" >&2
        exit 1
    }
}

# ── Versioning ───────────────────────────────────────────────────────────
latest_tag() {
    git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"
}

# Parses "vMAJOR.MINOR.PATCH" → "MAJOR MINOR PATCH" (space-separated).
parse_semver() {
    local v="${1#v}"
    local IFS=.
    # shellcheck disable=SC2086
    set -- $v
    if [[ $# -ne 3 ]] || ! [[ "$1" =~ ^[0-9]+$ && "$2" =~ ^[0-9]+$ && "$3" =~ ^[0-9]+$ ]]; then
        echo "error: cannot parse semver from '$1'" >&2
        return 1
    fi
    echo "$1 $2 $3"
}

bump_version() {
    local latest="$1" kind="$2"
    local parts
    parts="$(parse_semver "$latest")" || return 1
    # shellcheck disable=SC2086
    set -- $parts
    local major="$1" minor="$2" patch="$3"
    case "$kind" in
        major) major=$((major + 1)); minor=0; patch=0 ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        patch) patch=$((patch + 1)) ;;
        *) echo "error: unknown bump kind '$kind'" >&2; return 1 ;;
    esac
    echo "v${major}.${minor}.${patch}"
}

resolve_target() {
    local arg="$1" latest="$2"
    case "$arg" in
        patch|minor|major)
            bump_version "$latest" "$arg"
            ;;
        v[0-9]*.[0-9]*.[0-9]*)
            parse_semver "$arg" >/dev/null
            echo "$arg"
            ;;
        [0-9]*.[0-9]*.[0-9]*)
            parse_semver "v$arg" >/dev/null
            echo "v$arg"
            ;;
        *)
            echo "error: '$arg' is not a bump (patch/minor/major) or a version (vX.Y.Z)" >&2
            return 1
            ;;
    esac
}

changelog_since() {
    local prev="$1"
    if git rev-parse "$prev" >/dev/null 2>&1; then
        git log "$prev"..HEAD --pretty='- %s' --no-merges
    else
        git log --pretty='- %s' --no-merges
    fi
}

# ── Confirmation ─────────────────────────────────────────────────────────
confirm() {
    local prompt="$1"
    if [[ $ASSUME_YES -eq 1 ]]; then return 0; fi
    read -r -p "$prompt [y/N] " ans
    [[ "$ans" =~ ^[Yy]$ ]]
}

# ── Dry run ──────────────────────────────────────────────────────────────
run_dry() {
    require_goreleaser
    echo "  Running goreleaser in snapshot mode..."
    goreleaser release --snapshot --clean
    echo
    echo "  Snapshot artifacts in dist/. Inspect with: ls dist/"
    echo "  No tag was created. Re-run without --dry-run to ship for real."
}

# ── Real run ─────────────────────────────────────────────────────────────
run_real() {
    local tag="$1" body="$2"
    git tag -a "$tag" -m "$body"
    git push origin "$tag"
    echo
    echo "  ✓ Tagged and pushed $tag"
    echo
    echo "  Workflow:  https://github.com/${REPO_OWNER}/${REPO_NAME}/actions/workflows/release.yml"
    echo "  Release:   https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/tag/${tag}"
    echo
    echo "  CI is running goreleaser now (~90s). When it's done:"
    echo "    brew upgrade --cask agent-helper vault-helper sops-helper"
}

# ── Main ─────────────────────────────────────────────────────────────────
echo "── Releasing dotfiles-helpers ──"
echo

if [[ $DRY_RUN -eq 1 ]]; then
    echo "  Mode: DRY RUN (snapshot only)"
    require_tests_pass
    run_dry
    exit 0
fi

require_clean_repo
require_branch_main
require_synced
require_tests_pass

LATEST="$(latest_tag)"
NEXT="$(resolve_target "$TARGET" "$LATEST")"

if [[ "$NEXT" == "$LATEST" ]]; then
    echo "error: target version '$NEXT' equals latest tag — nothing to release" >&2
    exit 1
fi

echo
echo "  Latest tag:    $LATEST"
echo "  Next version:  $NEXT"
echo
echo "  Commits since $LATEST:"
changelog_since "$LATEST" | sed 's/^/    /'
echo

if ! confirm "  Tag $NEXT and push to origin?"; then
    echo "  Aborted."
    exit 0
fi

# Annotated tag with the changelog as the body.
TAG_BODY="$(printf '%s\n\nChanges:\n%s\n' "$NEXT" "$(changelog_since "$LATEST")")"
run_real "$NEXT" "$TAG_BODY"
