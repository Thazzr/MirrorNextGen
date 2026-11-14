#!/usr/bin/env bash
# Installs the host-side update service. Run with sudo from the mirror
# directory (the one holding compose.yaml and .env):
#   sudo ./host/install.sh [--with-timer]
# MIRROR_DIR may override the directory (default: current directory).
set -Eeuo pipefail

# Prints a unit file with @MIRROR_DIR@ replaced.
render_unit() {
  sed "s|@MIRROR_DIR@|$2|g" "$1"
}

main() {
  local with_timer=0 arg
  for arg in "$@"; do
    case "$arg" in
      --with-timer) with_timer=1 ;;
      *) echo "usage: install.sh [--with-timer]" >&2; exit 2 ;;
    esac
  done
  if [[ $EUID -ne 0 ]]; then
    echo "run with sudo" >&2
    exit 1
  fi

  local src mirror_dir
  src=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
  mirror_dir=$(cd "${MIRROR_DIR:-$PWD}" && pwd)
  if [[ ! -f "$mirror_dir/compose.yaml" ]]; then
    echo "no compose.yaml in $mirror_dir (run from the mirror directory)" >&2
    exit 1
  fi

  mkdir -p "$mirror_dir/host"
  if [[ "$src" != "$mirror_dir/host" ]]; then
    install -m 0755 "$src/mirror-update.sh" "$mirror_dir/host/mirror-update.sh"
  else
    chmod 0755 "$mirror_dir/host/mirror-update.sh"
  fi

  # The mirror container runs as nonroot (UID/GID 65532) and writes the request here.
  mkdir -p "$mirror_dir/control"
  chown 65532:65532 "$mirror_dir/control"
  chmod 0775 "$mirror_dir/control"

  local unit
  for unit in mirror-update.service mirror-update.path mirror-update.timer; do
    render_unit "$src/$unit" "$mirror_dir" >"/etc/systemd/system/$unit"
    chmod 0644 "/etc/systemd/system/$unit"
  done

  systemctl daemon-reload
  systemctl enable --now mirror-update.path
  if [[ $with_timer -eq 1 ]]; then
    systemctl enable --now mirror-update.timer
  fi
  echo "Installed. Mirror directory: $mirror_dir"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
