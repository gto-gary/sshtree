#!/bin/sh
# Install sshtree from a GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/gto-gary/sshtree/main/install.sh | sh
#
# Environment variables:
#   SSHTREE_VERSION      release tag to install (default: latest, e.g. v0.1.0)
#   SSHTREE_INSTALL_DIR  where to put the binary (default: ~/.local/bin)

set -eu

REPO="gto-gary/sshtree"

err() {
	echo "sshtree install: $*" >&2
	exit 1
}

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		err "curl or wget is required"
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		err "sha256sum or shasum is required to verify the download"
	fi
}

# Everything runs inside main so a partially downloaded script does nothing.
main() {
	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) err "unsupported OS: $(uname -s)" ;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) err "unsupported architecture: $(uname -m)" ;;
	esac

	version="${SSHTREE_VERSION:-latest}"
	if [ "$version" = latest ]; then
		base="https://github.com/$REPO/releases/latest/download"
	else
		base="https://github.com/$REPO/releases/download/$version"
	fi

	install_dir="${SSHTREE_INSTALL_DIR:-$HOME/.local/bin}"
	archive="sshtree_${os}_${arch}.tar.gz"

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	echo "Downloading $archive ($version)..."
	download "$base/$archive" "$tmp/$archive" || err "failed to download $base/$archive"
	download "$base/checksums.txt" "$tmp/checksums.txt" || err "failed to download checksums.txt"

	expected="$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
	[ -n "$expected" ] || err "no checksum listed for $archive"
	[ "$(sha256 "$tmp/$archive")" = "$expected" ] || err "checksum mismatch for $archive"

	tar -xzf "$tmp/$archive" -C "$tmp" sshtree

	mkdir -p "$install_dir"
	mv "$tmp/sshtree" "$install_dir/sshtree"
	chmod +x "$install_dir/sshtree"

	echo "Installed sshtree to $install_dir/sshtree"

	case ":$PATH:" in
	*":$install_dir:"*) ;;
	*) echo "Note: $install_dir is not on your PATH. Add it to your shell profile:"
	   echo "  export PATH=\"$install_dir:\$PATH\"" ;;
	esac
}

main "$@"
