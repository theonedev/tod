#!/usr/bin/env bash
set -euo pipefail

readonly BASE_URL="https://code.onedev.io/onedev/tod/~site"

detect_platform_arch() {
	local os arch

	case "$(uname -s)" in
	Linux*) os=linux ;;
	Darwin*) os=mac ;;
	FreeBSD*) os=freebsd ;;
	CYGWIN* | MINGW* | MSYS*) os=windows ;;
	*)
		echo "Unsupported operating system: $(uname -s)" >&2
		exit 1
		;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*)
		echo "Unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
	esac

	echo "${os}-${arch}"
}

binary_name() {
	local platform_arch=$1
	if [[ $platform_arch == windows-* ]]; then
		echo "tod.exe"
	else
		echo "tod"
	fi
}

install_dir() {
	local existing

	if existing=$(command -v tod 2>/dev/null); then
		dirname "$existing"
		return
	fi
	if existing=$(command -v tod.exe 2>/dev/null); then
		dirname "$existing"
		return
	fi

	if [[ -n ${INSTALL_DIR:-} ]]; then
		echo "$INSTALL_DIR"
		return
	fi

	if [[ :$PATH: == *":$HOME/.local/bin:"* ]]; then
		echo "$HOME/.local/bin"
		return
	fi

	if [[ -d /usr/local/bin && -w /usr/local/bin ]]; then
		echo "/usr/local/bin"
		return
	fi

	echo "$HOME/.local/bin"
}

download() {
	local url=$1 dest=$2

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$dest"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$dest" "$url"
	else
		echo "Neither curl nor wget is available; install one and retry." >&2
		exit 1
	fi
}

install_binary() {
	local dest_dir=$1 dest=$2 url=$3
	local tmp

	mkdir -p "$dest_dir"
	tmp=$(mktemp "${dest_dir}/.tod-install.XXXXXX")
	trap 'rm -f "$tmp"' EXIT

	echo "Downloading tod from ${url}..."
	download "$url" "$tmp"
	chmod +x "$tmp"
	mv -f "$tmp" "$dest"
	trap - EXIT

	echo "Installed tod to ${dest}"
}

main() {
	local platform_arch binary url dest_dir dest

	platform_arch=$(detect_platform_arch)
	binary=$(binary_name "$platform_arch")
	url="${BASE_URL}/${platform_arch}/${binary}"
	dest_dir=$(install_dir)
	dest="${dest_dir}/${binary}"

	if [[ ! -w $dest_dir ]]; then
		if command -v sudo >/dev/null 2>&1; then
			echo "Installing to ${dest_dir} (requires sudo)..."
			tmp=$(mktemp)
			trap 'rm -f "$tmp"' EXIT
			download "$url" "$tmp"
			sudo install -m 755 "$tmp" "$dest"
			rm -f "$tmp"
			trap - EXIT
			echo "Installed tod to ${dest}"
		else
			echo "Cannot write to ${dest_dir} and sudo is not available." >&2
			echo "Set INSTALL_DIR to a writable directory and retry." >&2
			exit 1
		fi
	else
		install_binary "$dest_dir" "$dest" "$url"
	fi

	if ! command -v tod >/dev/null 2>&1 && ! command -v tod.exe >/dev/null 2>&1; then
		echo
		echo "Add ${dest_dir} to your PATH, for example:"
		echo "  export PATH=\"${dest_dir}:\$PATH\""
	fi
}

main "$@"
