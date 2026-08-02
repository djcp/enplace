#!/bin/sh
# enplace installer — Linux & macOS
#
#   curl -fsSL https://raw.githubusercontent.com/djcp/enplace/main/install.sh | sh
#
# Environment overrides:
#   ENPLACE_VERSION   install a specific tag (e.g. v1.4.0-alpha); default: latest
#   INSTALL_DIR       install location; default: ~/.local/bin
#
# Downloads the matching release archive from GitHub, verifies its sha256 against
# the release checksums.txt, and installs the `enplace` binary.
set -eu

REPO="djcp/enplace"
BIN="enplace"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

info() { printf '%s\n' "$*" >&2; }
err() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

need() { command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"; }

need uname
need tar
need mkdir
# curl or wget for downloads
if command -v curl >/dev/null 2>&1; then
	DL="curl -fsSL"
	DLO="curl -fsSL -o"
elif command -v wget >/dev/null 2>&1; then
	DL="wget -qO-"
	DLO="wget -qO"
else
	err "need curl or wget"
fi

# --- Detect OS/arch -----------------------------------------------------------
os=$(uname -s)
case "$os" in
Linux) os="linux" ;;
Darwin) os="darwin" ;;
*) err "unsupported OS: $os (use the Windows installer or 'go install')" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch="amd64" ;;
aarch64 | arm64) arch="arm64" ;;
*) err "unsupported architecture: $arch" ;;
esac

# --- Resolve version ----------------------------------------------------------
api="https://api.github.com/repos/$REPO/releases"
if [ -n "${ENPLACE_VERSION:-}" ]; then
	tag="$ENPLACE_VERSION"
else
	# Prefer the latest stable release; fall back to the newest release of any
	# kind (this project currently ships prerelease/alpha tags).
	tag=$($DL "$api/latest" 2>/dev/null | grep -m1 '"tag_name":' | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/' || true)
	if [ -z "$tag" ]; then
		tag=$($DL "$api?per_page=1" 2>/dev/null | grep -m1 '"tag_name":' | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/' || true)
	fi
fi
[ -n "$tag" ] || err "could not determine the latest release; set ENPLACE_VERSION"

# Archive filename uses the version without a leading 'v'.
ver=${tag#v}
archive="${BIN}_${ver}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

info "Installing $BIN $tag ($os/$arch)…"

# --- Download + verify --------------------------------------------------------
tmp=$(mktemp -d 2>/dev/null || mktemp -d -t enplace)
trap 'rm -rf "$tmp"' EXIT INT TERM

$DLO "$tmp/$archive" "$base/$archive" || err "download failed: $base/$archive"
$DLO "$tmp/checksums.txt" "$base/checksums.txt" 2>/dev/null || info "warning: checksums.txt not found — skipping verification"

if [ -f "$tmp/checksums.txt" ]; then
	want=$(grep " $archive\$" "$tmp/checksums.txt" | awk '{print $1}')
	[ -n "$want" ] || err "no checksum entry for $archive"
	if command -v sha256sum >/dev/null 2>&1; then
		got=$(sha256sum "$tmp/$archive" | awk '{print $1}')
	elif command -v shasum >/dev/null 2>&1; then
		got=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
	else
		got=""
		info "warning: no sha256 tool found — skipping verification"
	fi
	if [ -n "$got" ] && [ "$got" != "$want" ]; then
		err "checksum mismatch for $archive (expected $want, got $got)"
	fi
fi

# --- Install ------------------------------------------------------------------
tar -xzf "$tmp/$archive" -C "$tmp" "$BIN" || err "failed to extract $BIN from archive"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BIN" "$INSTALL_DIR/$BIN" 2>/dev/null ||
	{ cp "$tmp/$BIN" "$INSTALL_DIR/$BIN" && chmod 0755 "$INSTALL_DIR/$BIN"; }

info ""
info "✓ Installed $BIN to $INSTALL_DIR/$BIN"

case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*)
	info ""
	info "$INSTALL_DIR is not on your PATH. Add it, e.g.:"
	info "    echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.profile"
	;;
esac

info ""
info "Run 'enplace' to get started, and 'enplace update' to upgrade later."
