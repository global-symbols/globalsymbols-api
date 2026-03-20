#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install a previously uploaded release into the final runtime location.

Run this on the target server (pre-production or production) after CI has copied
the release artifacts to the configured upload directory.

Usage:
  ./install_release.sh --environment {pre-production|production} --release-id <RELEASE_ID>

Required environment variables (depending on --environment):

  pre-production:
    PREPRODUCTION_UPLOAD_DIR
    PREPRODUCTION_INSTALL_DIR
    PREPRODUCTION_SERVICE_NAME
    PREPRODUCTION_ENV_FILE

  production:
    PRODUCTION_UPLOAD_DIR
    PRODUCTION_INSTALL_DIR
    PRODUCTION_SERVICE_NAME
    PRODUCTION_ENV_FILE

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

case "${ENVIRONMENT}" in
  pre-production)
    : "${PREPRODUCTION_UPLOAD_DIR:?PREPRODUCTION_UPLOAD_DIR is required}"
    : "${PREPRODUCTION_INSTALL_DIR:?PREPRODUCTION_INSTALL_DIR is required}"
    : "${PREPRODUCTION_SERVICE_NAME:?PREPRODUCTION_SERVICE_NAME is required}"
    : "${PREPRODUCTION_ENV_FILE:?PREPRODUCTION_ENV_FILE is required}"
    TARGET_UPLOAD_DIR="${PREPRODUCTION_UPLOAD_DIR}"
    FINAL_INSTALL_DIR="${PREPRODUCTION_INSTALL_DIR}"
    SERVICE_NAME="${PREPRODUCTION_SERVICE_NAME}"
    ENV_FILE="${PREPRODUCTION_ENV_FILE}"
    ;;
  production)
    : "${PRODUCTION_UPLOAD_DIR:?PRODUCTION_UPLOAD_DIR is required}"
    : "${PRODUCTION_INSTALL_DIR:?PRODUCTION_INSTALL_DIR is required}"
    : "${PRODUCTION_SERVICE_NAME:?PRODUCTION_SERVICE_NAME is required}"
    : "${PRODUCTION_ENV_FILE:?PRODUCTION_ENV_FILE is required}"
    TARGET_UPLOAD_DIR="${PRODUCTION_UPLOAD_DIR}"
    FINAL_INSTALL_DIR="${PRODUCTION_INSTALL_DIR}"
    SERVICE_NAME="${PRODUCTION_SERVICE_NAME}"
    ENV_FILE="${PRODUCTION_ENV_FILE}"
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

log "Deployment complete. Current -> ${CURRENT_LINK}"

