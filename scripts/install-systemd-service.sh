#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="bm-gpu-process-exporter"
SERVICE_USER="bm-gpu-exporter"
SERVICE_GROUP="bm-gpu-exporter"
INSTALL_BIN="/usr/local/bin/${SERVICE_NAME}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
ENV_FILE="/etc/default/${SERVICE_NAME}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
UNIT_SOURCE="${REPO_ROOT}/deploy/systemd/${SERVICE_NAME}.service"

if [[ "${EUID}" -ne 0 ]]; then
  echo "This installer must run as root. Example:"
  echo "  sudo $0 /path/to/${SERVICE_NAME}"
  exit 1
fi

if [[ ! -f "${UNIT_SOURCE}" ]]; then
  echo "Service template not found: ${UNIT_SOURCE}"
  exit 1
fi

SOURCE_BIN="${1:-}"
if [[ -z "${SOURCE_BIN}" ]]; then
  if [[ -x "${REPO_ROOT}/bin/${SERVICE_NAME}" ]]; then
    SOURCE_BIN="${REPO_ROOT}/bin/${SERVICE_NAME}"
  elif [[ -x "${REPO_ROOT}/bin/bm-gpu-task-exporter-linux-amd64" ]]; then
    SOURCE_BIN="${REPO_ROOT}/bin/bm-gpu-task-exporter-linux-amd64"
  else
    echo "No source binary provided and no default binary found."
    echo "Pass an explicit path, for example:"
    echo "  sudo $0 ${REPO_ROOT}/bin/${SERVICE_NAME}"
    exit 1
  fi
fi

if [[ ! -f "${SOURCE_BIN}" ]]; then
  echo "Binary not found: ${SOURCE_BIN}"
  exit 1
fi

if [[ ! -x "${SOURCE_BIN}" ]]; then
  echo "Binary exists but is not executable: ${SOURCE_BIN}"
  exit 1
fi

if ! getent group "${SERVICE_GROUP}" >/dev/null; then
  groupadd --system "${SERVICE_GROUP}"
fi

if ! id -u "${SERVICE_USER}" >/dev/null 2>&1; then
  NOLOGIN_BIN="/usr/sbin/nologin"
  if [[ ! -x "${NOLOGIN_BIN}" ]]; then
    NOLOGIN_BIN="/sbin/nologin"
  fi
  useradd \
    --system \
    --gid "${SERVICE_GROUP}" \
    --no-create-home \
    --shell "${NOLOGIN_BIN}" \
    "${SERVICE_USER}"
fi

# Add the service user to video group when present so nvidia-smi can access GPU metadata.
if getent group video >/dev/null; then
  usermod -aG video "${SERVICE_USER}" || true
fi

install -D -m 0755 "${SOURCE_BIN}" "${INSTALL_BIN}"
install -D -m 0644 "${UNIT_SOURCE}" "${SERVICE_FILE}"

if [[ ! -f "${ENV_FILE}" ]]; then
  cat > "${ENV_FILE}" <<'EOF'
# bm-gpu-process-exporter runtime config
# HOST=0.0.0.0
# PORT=9101
# UPDATE_INTERVAL_SECONDS=5
EOF
  chmod 0644 "${ENV_FILE}"
fi

systemctl daemon-reload
systemctl enable --now "${SERVICE_NAME}.service"

echo "Installed ${SERVICE_NAME}."
echo "Service file: ${SERVICE_FILE}"
echo "Env file: ${ENV_FILE}"
echo "Binary: ${INSTALL_BIN}"
echo
echo "Check status with:"
echo "  systemctl status ${SERVICE_NAME}.service --no-pager"
