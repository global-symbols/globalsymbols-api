#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install a previously uploaded release into the final runtime location.

Run this on the target server (pre-production or production) after CI has copied
the release artifacts to the configured upload directory.

Usage:
  ./install_release.sh --environment {pre-production|production} --release-id <RELEASE_ID>

Optional environment variable overrides (depending on --environment):

  pre-production:
    PREPRODUCTION_UPLOAD_DIR    default: /var/www/globalsymbols-api/uploads
    PREPRODUCTION_INSTALL_DIR   default: /var/www/globalsymbols-api
    PREPRODUCTION_SERVICE_NAME  default: globalsymbols-api.service
    PREPRODUCTION_ENV_FILE      default: /var/www/globalsymbols-api/.env

  production:
    PRODUCTION_UPLOAD_DIR       default: /var/www/globalsymbols-api/uploads
    PRODUCTION_INSTALL_DIR      default: /var/www/globalsymbols-api
    PRODUCTION_SERVICE_NAME     default: globalsymbols-api.service
    PRODUCTION_ENV_FILE         default: /var/www/globalsymbols-api/.env

  shared:
    UPLOAD_RELEASES_TO_KEEP     default: 5

Notes:
  - RELEASE_ID must match the directory name created by CI on the server.
  - The runtime environment file must already exist and live outside the release directory.
  - This script updates:
      <FINAL_INSTALL_DIR>/releases/<RELEASE_ID>
      and a symlink at <FINAL_INSTALL_DIR>/current -> releases/<RELEASE_ID>
  - The service is restarted via:
      systemctl restart <SERVICE_NAME>
EOF
}

log() {
  echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"
}

cleanup_old_uploads() {
  local upload_dir="$1"
  local keep_count="$2"

  if [[ ! "${keep_count}" =~ ^[0-9]+$ ]]; then
    echo "UPLOAD_RELEASES_TO_KEEP must be a non-negative integer, got: ${keep_count}" >&2
    exit 2
  fi

  if [[ ! -d "${upload_dir}" ]]; then
    log "Upload cleanup skipped: upload directory not found (${upload_dir})"
    return
  fi

  local -a upload_dirs=()
  mapfile -t upload_dirs < <(find "${upload_dir}" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %p\n' | sort -rn | awk '{ $1=""; sub(/^ /, ""); print }')

  if (( ${#upload_dirs[@]} <= keep_count )); then
    log "Upload cleanup skipped: ${#upload_dirs[@]} staged release(s), retention is ${keep_count}"
    return
  fi

  local index=0
  local removed=0
  for dir in "${upload_dirs[@]}"; do
    (( index += 1 ))
    if (( index <= keep_count )); then
      continue
    fi

    rm -rf "${dir}"
    (( removed += 1 ))
    log "Removed old uploaded release: ${dir}"
  done

  log "Upload cleanup complete. Retained newest ${keep_count} staged release(s); removed ${removed}."
}

SUDO_BIN="${SUDO_BIN:-}"
if [[ -z "${SUDO_BIN}" ]]; then
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    SUDO_BIN=""
  else
    SUDO_BIN="sudo"
  fi
fi

ENVIRONMENT=""
RELEASE_ID=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --environment|-e)
      ENVIRONMENT="${2:-}"
      shift 2
      ;;
    --release-id|-r)
      RELEASE_ID="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${ENVIRONMENT}" || -z "${RELEASE_ID}" ]]; then
  usage >&2
  exit 2
fi

TARGET_UPLOAD_DIR=""
FINAL_INSTALL_DIR=""
SERVICE_NAME=""
ENV_FILE=""
UPLOAD_RELEASES_TO_KEEP="${UPLOAD_RELEASES_TO_KEEP:-5}"

case "${ENVIRONMENT}" in
  pre-production)
    TARGET_UPLOAD_DIR="${PREPRODUCTION_UPLOAD_DIR:-/var/www/globalsymbols-api/uploads}"
    FINAL_INSTALL_DIR="${PREPRODUCTION_INSTALL_DIR:-/var/www/globalsymbols-api}"
    SERVICE_NAME="${PREPRODUCTION_SERVICE_NAME:-globalsymbols-api.service}"
    ENV_FILE="${PREPRODUCTION_ENV_FILE:-/var/www/globalsymbols-api/.env}"
    ;;
  production)
    TARGET_UPLOAD_DIR="${PRODUCTION_UPLOAD_DIR:-/var/www/globalsymbols-api/uploads}"
    FINAL_INSTALL_DIR="${PRODUCTION_INSTALL_DIR:-/var/www/globalsymbols-api}"
    SERVICE_NAME="${PRODUCTION_SERVICE_NAME:-globalsymbols-api.service}"
    ENV_FILE="${PRODUCTION_ENV_FILE:-/var/www/globalsymbols-api/.env}"
    ;;
  *)
    echo "Invalid --environment: ${ENVIRONMENT}. Expected pre-production or production." >&2
    exit 2
    ;;
esac

SOURCE_RELEASE_DIR="${TARGET_UPLOAD_DIR}/${RELEASE_ID}"
SOURCE_BIN="${SOURCE_RELEASE_DIR}/bin/go-api"

DEST_RELEASE_DIR="${FINAL_INSTALL_DIR}/releases/${RELEASE_ID}"
CURRENT_LINK="${FINAL_INSTALL_DIR}/current"

if [[ ! -d "${SOURCE_RELEASE_DIR}" ]]; then
  echo "Release directory not found: ${SOURCE_RELEASE_DIR}" >&2
  exit 1
fi
if [[ ! -f "${SOURCE_BIN}" ]]; then
  echo "Release binary not found: ${SOURCE_BIN}" >&2
  exit 1
fi
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Environment file not found: ${ENV_FILE}" >&2
  exit 1
fi

log "Installing release ${RELEASE_ID} (${ENVIRONMENT})"
log "Upload source: ${SOURCE_RELEASE_DIR}"
log "Install target: ${DEST_RELEASE_DIR}"

mkdir -p "${FINAL_INSTALL_DIR}/releases"

# Re-installing the same RELEASE_ID should be safe: overwrite only that release dir.
rm -rf "${DEST_RELEASE_DIR}"
mkdir -p "${DEST_RELEASE_DIR}"

if command -v rsync >/dev/null 2>&1; then
  rsync -a --delete "${SOURCE_RELEASE_DIR}/" "${DEST_RELEASE_DIR}/"
else
  # Fallback if rsync isn't available.
  cp -a "${SOURCE_RELEASE_DIR}/." "${DEST_RELEASE_DIR}/"
fi

chmod +x "${DEST_RELEASE_DIR}/bin/go-api"

# Atomic-ish cutover: update the symlink to point at the new release.
ln -sfn "${DEST_RELEASE_DIR}" "${CURRENT_LINK}"

log "Restarting service: ${SERVICE_NAME}"
if [[ -n "${SUDO_BIN}" ]]; then
  if ! command -v "${SUDO_BIN}" >/dev/null 2>&1; then
    echo "Sudo requested but not found: ${SUDO_BIN}" >&2
    exit 1
  fi
fi

${SUDO_BIN} systemctl restart "${SERVICE_NAME}"
${SUDO_BIN} systemctl is-active --quiet "${SERVICE_NAME}"

cleanup_old_uploads "${TARGET_UPLOAD_DIR}" "${UPLOAD_RELEASES_TO_KEEP}"

log "Deployment complete. Current -> ${CURRENT_LINK}"

