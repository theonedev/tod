#!/usr/bin/env bash
# publish.sh — build the tod CLI and install it (and the bundled skills)
# globally on this machine.
#
# Two things are installed by default:
#   1. The `tod` binary (built via `go build`) is symlinked into ~/.local/bin.
#   2. Each `skills/<name>/` folder containing a `SKILL.md` is symlinked into
#      Cursor's global skills directory.
#
# Symlinks are used by default so edits in this repo are picked up live.
# Pass `--copy` to copy real files/folders instead.
#
# Usage:
#   ./publish.sh                  # build + symlink binary and skills
#   ./publish.sh --copy           # copy instead of symlinking
#   ./publish.sh --force          # overwrite existing destinations
#   ./publish.sh --dest <path>    # override skills destination
#   ./publish.sh --bin-dest <p>   # override binary destination dir (default ~/.local/bin)
#   ./publish.sh --skip-binary    # only install skills
#   ./publish.sh --skip-skills    # only build + install the binary
#   ./publish.sh --dry-run        # print what would happen, don't touch the FS
#   ./publish.sh -h | --help      # show help

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SRC_DIR="${SCRIPT_DIR}/skills"
BIN_SRC="${SCRIPT_DIR}/tod"

mode="symlink"
force=0
dry_run=0
skip_binary=0
skip_skills=0
dest=""
bin_dest=""

usage() {
    awk '
        NR == 1 { next }                       # skip shebang
        /^#/    { sub(/^# ?/, ""); print; next }
        { exit }                               # stop at first non-comment line
    ' "${BASH_SOURCE[0]}"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --copy)    mode="copy"; shift ;;
        --symlink) mode="symlink"; shift ;;
        --force|-f) force=1; shift ;;
        --dry-run|-n) dry_run=1; shift ;;
        --skip-binary) skip_binary=1; shift ;;
        --skip-skills) skip_skills=1; shift ;;
        --dest)
            [[ $# -ge 2 ]] || { echo "error: --dest requires an argument" >&2; exit 2; }
            dest="$2"; shift 2 ;;
        --dest=*) dest="${1#--dest=}"; shift ;;
        --bin-dest)
            [[ $# -ge 2 ]] || { echo "error: --bin-dest requires an argument" >&2; exit 2; }
            bin_dest="$2"; shift 2 ;;
        --bin-dest=*) bin_dest="${1#--bin-dest=}"; shift ;;
        -h|--help) usage; exit 0 ;;
        *) echo "error: unknown argument: $1" >&2; usage >&2; exit 2 ;;
    esac
done

if [[ -z "$dest" ]]; then
    if [[ -d "$HOME/.cursor/skills-cursor" ]]; then
        dest="$HOME/.cursor/skills-cursor"
    elif [[ -d "$HOME/.cursor/skills" ]]; then
        dest="$HOME/.cursor/skills"
    else
        dest="$HOME/.cursor/skills-cursor"
    fi
fi

if [[ -z "$bin_dest" ]]; then
    bin_dest="$HOME/.local/bin"
fi

run() {
    if (( dry_run )); then
        printf 'DRY-RUN: %s\n' "$*"
    else
        "$@"
    fi
}

mkdir_p() {
    if [[ ! -d "$1" ]]; then
        run mkdir -p "$1"
    fi
}

# install_path <src> <dst>  honors $mode and $force/$dry_run.
install_path() {
    local src="$1" dst="$2"

    if [[ -e "$dst" || -L "$dst" ]]; then
        if (( force )); then
            run rm -rf "$dst"
        else
            echo "  skip   $(basename "$dst")  (already exists; use --force to overwrite)"
            return 1
        fi
    fi

    case "$mode" in
        symlink) run ln -s "$src" "$dst"; echo "  link   $(basename "$dst") -> $src" ;;
        copy)    run cp -R "$src" "$dst"; echo "  copy   $(basename "$dst") -> $dst" ;;
    esac
    return 0
}

echo "Mode   : $mode"
(( force ))       && echo "Force  : yes"
(( dry_run ))     && echo "DryRun : yes"
(( skip_binary )) && echo "Binary : skipped"
(( skip_skills )) && echo "Skills : skipped"
echo

bin_installed=0
skills_installed=0
skills_skipped=0

if (( ! skip_binary )); then
    echo "== Build & install tod binary =="
    echo "  Repo   : $SCRIPT_DIR"
    echo "  Dest   : $bin_dest"

    if ! command -v go >/dev/null 2>&1; then
        echo "error: 'go' is not on PATH; cannot build tod" >&2
        exit 1
    fi

    (
        cd "$SCRIPT_DIR"
        run go build -o "$BIN_SRC" .
    )

    if (( ! dry_run )) && [[ ! -x "$BIN_SRC" ]]; then
        echo "error: build did not produce $BIN_SRC" >&2
        exit 1
    fi

    mkdir_p "$bin_dest"
    if install_path "$BIN_SRC" "$bin_dest/tod"; then
        bin_installed=1
    fi

    case ":${PATH}:" in
        *":${bin_dest}:"*) ;;
        *) echo "  note   $bin_dest is not on your PATH" ;;
    esac
    echo
fi

if (( ! skip_skills )); then
    echo "== Install skills =="
    echo "  Source : $SKILLS_SRC_DIR"
    echo "  Dest   : $dest"

    if [[ ! -d "$SKILLS_SRC_DIR" ]]; then
        echo "error: skills source dir not found: $SKILLS_SRC_DIR" >&2
        exit 1
    fi

    mkdir_p "$dest"

    shopt -s nullglob
    for skill_dir in "$SKILLS_SRC_DIR"/*/; do
        skill_dir="${skill_dir%/}"
        name="$(basename "$skill_dir")"

        if [[ ! -f "$skill_dir/SKILL.md" ]]; then
            echo "  skip   $name  (no SKILL.md)"
            skills_skipped=$((skills_skipped + 1))
            continue
        fi

        if install_path "$skill_dir" "$dest/$name"; then
            skills_installed=$((skills_installed + 1))
        else
            skills_skipped=$((skills_skipped + 1))
        fi
    done
    echo
fi

echo "Done."
(( ! skip_binary )) && echo "  Binary : $([[ $bin_installed -eq 1 ]] && echo installed || echo skipped)"
(( ! skip_skills )) && echo "  Skills : $skills_installed installed, $skills_skipped skipped"

exit 0
