#!/usr/bin/env bash
# Host-side updater: pulls the new images and restarts the stack.
# Triggered by mirror-update.path when the mirror container writes
# <control>/update-request. Configured only through MIRROR_DIR.
set -Eeuo pipefail

if [[ -z "${MIRROR_DIR:-}" ]]; then
  echo "MIRROR_DIR is not set" >&2
  exit 2
fi
if [[ ! -f "$MIRROR_DIR/compose.yaml" ]]; then
  echo "MIRROR_DIR has no compose.yaml: $MIRROR_DIR" >&2
  exit 2
fi

control="$MIRROR_DIR/control"
status="$control/update-status.json"
request="$control/update-request"
log="$control/update.log"
mkdir -p "$control"

# Only one update at a time; a second trigger is a no-op.
exec 9>"$control/.update.lock"
if ! flock -n 9; then
  exit 0
fi

# The request is always consumed, otherwise the path unit would loop.
trap 'rm -f "$request"' EXIT

now() { date +%Y-%m-%dT%H:%M:%S%:z; }

# Escapes $1 for use inside a JSON string (without the quotes).
json_escape() {
  local s=$1
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  s=${s//$'\t'/\\t}
  s=${s//$'\r'/\\r}
  s=${s//$'\n'/\\n}
  # Drop remaining control characters, which are invalid in JSON strings.
  s=$(printf '%s' "$s" | tr -d '\000-\010\013\014\016-\037')
  printf '%s' "$s"
}

# Atomic write: temp file in the same directory, then rename.
write_status() {
  local tmp
  tmp=$(mktemp "$control/.update-status.XXXXXX")
  printf '%s\n' "$1" >"$tmp"
  chmod 0644 "$tmp"
  mv -f "$tmp" "$status"
}

started=$(now)
write_status "{\"state\":\"running\",\"startedAt\":\"$started\"}"

compose=(docker compose --project-directory "$MIRROR_DIR" -f "$MIRROR_DIR/compose.yaml" --env-file "$MIRROR_DIR/.env")

: >"$log"
if "${compose[@]}" pull >>"$log" 2>&1 && "${compose[@]}" up -d --remove-orphans >>"$log" 2>&1; then
  write_status "{\"state\":\"succeeded\",\"startedAt\":\"$started\",\"finishedAt\":\"$(now)\",\"message\":\"Aktualisiert.\"}"
  exit 0
fi

msg=$(tail -n 5 "$log" || true)
write_status "{\"state\":\"failed\",\"startedAt\":\"$started\",\"finishedAt\":\"$(now)\",\"message\":\"$(json_escape "$msg")\"}"
exit 1
