#!/bin/bash
# ==============================================================================
# domus Declarative Dashboards Automation Tool
# Manages local pre-commit previews, validation, and production deployments
# via the Grafana Foundation SDK (Go) and gcx CLI.
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DASHBOARDS_DIR="${ROOT_DIR}/dashboards"
GCX_CONTEXT="domus"
GRAFANA_BASE_URL="https://domus.grafana.net"

CLEANUP_DIRS=()
cleanup() {
  if [ "${#CLEANUP_DIRS[@]}" -gt 0 ]; then
    for d in "${CLEANUP_DIRS[@]}"; do
      if [ -n "$d" ] && [ -d "$d" ]; then
        rm -rf "$d"
      fi
    done
  fi
}
trap cleanup EXIT

# Color helpers
info() { echo -e "\033[1;34m[INFO]\033[0m $*"; }
success() { echo -e "\033[1;32m[SUCCESS]\033[0m $*"; }
warn() { echo -e "\033[1;33m[WARN]\033[0m $*"; }
error() { echo -e "\033[1;31m[ERROR]\033[0m $*" >&2; }

usage() {
  cat <<EOF
Usage: $(basename "$0") <command> [options]

Commands:
  preview [branch]    Generate and push dashboards to temporary folder domus-prev-<branch>
  clean-preview [branch] Delete temporary preview folder and dashboards from Grafana Cloud
  test-live [branch]  Deploy preview, run automated live non-regression tests, and cleanup
  deploy              Deploy production dashboards to primary folder domus
  validate            Run Go unit tests and validate generated manifests
  list                List all dashboards defined in the Go suite

Options:
  --verify-data       Verify live telemetry and variable options after pushing preview
  --host <host>       Target host for validation (default: kodlix.delarche.fr)
  --dry-run           Simulate push operations without creating or updating cloud resources
  -h, --help          Show this help message
EOF
  exit 1
}

get_current_branch() {
  if git rev-parse --git-dir >/dev/null 2>&1; then
    git rev-parse --abbrev-ref HEAD
  else
    echo "local"
  fi
}

cmd_preview() {
  local branch="${1:-$(get_current_branch)}"
  local dry_run="${2:-false}"
  local verify_data="${3:-false}"
  local host="${4:-kodlix.delarche.fr}"
  local tmp_dir
  tmp_dir=$(mktemp -d)
  CLEANUP_DIRS+=("${tmp_dir}")

  info "Generating preview manifests for branch: ${branch}"
  (cd "${DASHBOARDS_DIR}" && go run . -env preview -branch "${branch}" -out "${tmp_dir}")

  info "Validating manifests against Grafana Cloud (${GCX_CONTEXT} context)..."
  gcx --context "${GCX_CONTEXT}" resources validate -p "${tmp_dir}"

  if [ "${dry_run}" = "true" ]; then
    info "Dry-run enabled: skipping push to Grafana Cloud."
    success "Preview manifests validated successfully in dry-run mode."
    return 0
  fi

  info "Pushing preview dashboards and folder to Grafana Cloud..."
  gcx --context "${GCX_CONTEXT}" resources push -p "${tmp_dir}" --on-error abort

  local folder_uid
  folder_uid=$(cd "${DASHBOARDS_DIR}" && go run . -env preview -branch "${branch}" -folder-info | grep FOLDER_UID= | cut -d= -f2)
  local slug="${folder_uid#domus-prev-}"

  echo ""
  success "Preview dashboards deployed successfully!"
  info "Folder URL:     ${GRAFANA_BASE_URL}/dashboards/f/${folder_uid}"
  info "Host Overview:  ${GRAFANA_BASE_URL}/d/domus-host-p-${slug}"
  info "Storage Health: ${GRAFANA_BASE_URL}/d/domus-storage-p-${slug}"
  info "Backup Health:  ${GRAFANA_BASE_URL}/d/domus-backups-p-${slug}"
  info "Alloy Health:   ${GRAFANA_BASE_URL}/d/domus-alloy-health-p-${slug}"
  echo ""

  if [ "${verify_data}" = "true" ]; then
    info "Executing live telemetry verification (--verify-data)..."
    python3 "${SCRIPT_DIR}/verify_live.py" --branch "${branch}" --host "${host}"
  fi

  info "When finished, purge this preview folder by running:"
  info "  ./scripts/dashboards.sh clean-preview \"${branch}\""
}

cmd_clean_preview() {
  local branch="${1:-$(get_current_branch)}"
  local folder_uid
  folder_uid=$(cd "${DASHBOARDS_DIR}" && go run . -env preview -branch "${branch}" -folder-info | grep FOLDER_UID= | cut -d= -f2)

  info "Deleting preview folder ${folder_uid} from Grafana Cloud (${GCX_CONTEXT} context)..."
  if gcx --context "${GCX_CONTEXT}" api "/api/folders/${folder_uid}" -X DELETE >/dev/null 2>&1; then
    success "Preview folder ${folder_uid} deleted successfully."
  else
    warn "Folder ${folder_uid} was not found or already deleted."
  fi
}

cmd_test_live() {
  local branch="${1:-$(get_current_branch)}"
  local host="${2:-kodlix.delarche.fr}"

  info "Executing automated live test campaign on branch: ${branch}..."
  # Clean up existing preview if present
  cmd_clean_preview "${branch}" >/dev/null 2>&1 || true

  # Deploy preview
  cmd_preview "${branch}" "false" "false" "${host}"

  # Ensure clean preview deletion on exit
  # shellcheck disable=SC2064
  trap "cmd_clean_preview \"${branch}\"; cleanup" EXIT

  # Run verification
  python3 "${SCRIPT_DIR}/verify_live.py" --branch "${branch}" --host "${host}"
  success "Live non-regression test campaign passed completely!"
}

cmd_deploy() {
  local dry_run="${1:-false}"
  local tmp_dir
  tmp_dir=$(mktemp -d)
  CLEANUP_DIRS+=("${tmp_dir}")

  info "Generating production manifests for domus folder..."
  (cd "${DASHBOARDS_DIR}" && go run . -env prod -out "${tmp_dir}")

  info "Validating manifests against Grafana Cloud (${GCX_CONTEXT} context)..."
  gcx --context "${GCX_CONTEXT}" resources validate -p "${tmp_dir}"

  if [ "${dry_run}" = "true" ]; then
    info "Dry-run enabled: skipping production push."
    success "Production manifests validated successfully in dry-run mode."
    return 0
  fi

  info "Deploying production dashboards to Grafana Cloud..."
  gcx --context "${GCX_CONTEXT}" resources push -p "${tmp_dir}" --on-error abort

  echo ""
  success "Production dashboards deployed successfully to folder 'domus'!"
  info "Folder URL:     ${GRAFANA_BASE_URL}/dashboards/f/domus"
  info "Host Overview:  ${GRAFANA_BASE_URL}/d/domus-host"
  info "Storage Health: ${GRAFANA_BASE_URL}/d/domus-storage"
  info "Backup Health:  ${GRAFANA_BASE_URL}/d/domus-backups"
  info "Alloy Health:   ${GRAFANA_BASE_URL}/d/domus-alloy-health"
}

cmd_validate() {
  info "Running Go unit tests in dashboards..."
  (cd "${DASHBOARDS_DIR}" && go test -v ./...)

  local tmp_dir
  tmp_dir=$(mktemp -d)
  CLEANUP_DIRS+=("${tmp_dir}")

  local has_gcx="false"
  if command -v gcx >/dev/null 2>&1 && gcx config check --context "${GCX_CONTEXT}" >/dev/null 2>&1; then
    has_gcx="true"
  else
    warn "gcx context '${GCX_CONTEXT}' not available; running local manifest generation checks only."
  fi

  info "Generating and validating production manifests..."
  (cd "${DASHBOARDS_DIR}" && go run . -env prod -out "${tmp_dir}")
  if [ "${has_gcx}" = "true" ]; then
    gcx --context "${GCX_CONTEXT}" resources validate -p "${tmp_dir}"
  fi

  info "Generating and validating preview manifests..."
  (cd "${DASHBOARDS_DIR}" && go run . -env preview -branch "ci-test" -out "${tmp_dir}")
  if [ "${has_gcx}" = "true" ]; then
    gcx --context "${GCX_CONTEXT}" resources validate -p "${tmp_dir}"
  fi

  success "All dashboard definitions and manifests are valid!"
}

cmd_list() {
  (cd "${DASHBOARDS_DIR}" && go run . -list)
}

# Entrypoint routing
main() {
  if [ $# -eq 0 ]; then
    usage
  fi

  local action="$1"
  shift

  case "${action}" in
    preview)
      local branch=""
      local dry_run="false"
      local verify_data="false"
      local host="kodlix.delarche.fr"
      while [ $# -gt 0 ]; do
        case "$1" in
          --dry-run) dry_run="true" ;;
          --verify-data) verify_data="true" ;;
          --host) shift; host="$1" ;;
          -*) error "Unknown option: $1"; usage ;;
          *) branch="$1" ;;
        esac
        shift
      done
      cmd_preview "${branch}" "${dry_run}" "${verify_data}" "${host}"
      ;;
    clean-preview)
      local branch="${1:-}"
      cmd_clean_preview "${branch}"
      ;;
    test-live)
      local branch=""
      local host="kodlix.delarche.fr"
      while [ $# -gt 0 ]; do
        case "$1" in
          --host) shift; host="$1" ;;
          -*) error "Unknown option: $1"; usage ;;
          *) branch="$1" ;;
        esac
        shift
      done
      cmd_test_live "${branch}" "${host}"
      ;;
    deploy)
      local dry_run="false"
      while [ $# -gt 0 ]; do
        case "$1" in
          --dry-run) dry_run="true" ;;
          -*) error "Unknown option: $1"; usage ;;
        esac
        shift
      done
      cmd_deploy "${dry_run}"
      ;;
    validate)
      cmd_validate
      ;;
    list)
      cmd_list
      ;;
    -h|--help)
      usage
      ;;
    *)
      error "Unknown command: ${action}"
      usage
      ;;
  esac
}

main "$@"
