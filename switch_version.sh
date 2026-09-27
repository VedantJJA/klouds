#!/usr/bin/env bash
# switch_version.sh — Switch the Klouds PaaS between git versions.
#
# Usage:
#   ./switch_version.sh <commit-hash|tag|branch>
#   ./switch_version.sh --current          # Show what's running
#   ./switch_version.sh --list             # List recent commits
#
# Examples:
#   ./switch_version.sh 49d93bb            # Switch to specific commit
#   ./switch_version.sh v1.0.0             # Switch to a tag
#   ./switch_version.sh main               # Switch to branch tip
#   ./switch_version.sh f64d532            # Roll back to commit 010

set -euo pipefail

REPO_DIR="/home/ubuntu/klouds"
SERVICE_API="klouds.service"
SERVICE_DASHBOARD="klouds-dashboard.service"
DASHBOARD_DIR="$REPO_DIR/dashboard"
BUILD_LOG="/var/log/klouds-switch.log"

# ── Colours ────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; NC='\033[0m'; BOLD='\033[1m'

log()  { echo -e "${CYAN}[klouds-switch]${NC} $*"; }
ok()   { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*" >&2; exit 1; }

# ── Sanity checks ──────────────────────────────────────────────────────────
[[ $EUID -ne 0 ]] && { SUDO="sudo"; } || SUDO=""

# ── Argument parsing ───────────────────────────────────────────────────────
if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <commit-hash|tag|branch>"
  echo "       $0 --current    (show running version)"
  echo "       $0 --list       (list recent commits)"
  exit 1
fi

TARGET="$1"

# ── --current ──────────────────────────────────────────────────────────────
if [[ "$TARGET" == "--current" ]]; then
  cd "$REPO_DIR"
  CURRENT=$(git rev-parse --short HEAD)
  BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "detached")
  SUBJECT=$(git log -1 --pretty=format:'%s')
  echo -e "${BOLD}Running version:${NC} ${GREEN}$CURRENT${NC} ($BRANCH)"
  echo -e "${BOLD}Commit message:${NC}  $SUBJECT"
  echo ""
  echo "Services:"
  systemctl is-active $SERVICE_API      && ok "$SERVICE_API active"      || warn "$SERVICE_API NOT active"
  systemctl is-active $SERVICE_DASHBOARD && ok "$SERVICE_DASHBOARD active" || warn "$SERVICE_DASHBOARD NOT active"
  exit 0
fi

# ── --list ─────────────────────────────────────────────────────────────────
if [[ "$TARGET" == "--list" ]]; then
  cd "$REPO_DIR"
  git fetch --quiet origin 2>/dev/null || true
  echo -e "${BOLD}Recent commits (newest first):${NC}"
  git log --oneline -20 2>/dev/null
  exit 0
fi

# ── Main switch flow ───────────────────────────────────────────────────────
log "Target: ${BOLD}$TARGET${NC}"
log "Build log: $BUILD_LOG"

cd "$REPO_DIR"

# Save current HEAD so we can roll back on failure
PREVIOUS=$(git rev-parse --short HEAD)
log "Current HEAD: $PREVIOUS"

# Fetch latest refs
log "Fetching from origin..."
git fetch --all --tags --quiet

# Resolve target (commit, tag, or branch)
if ! RESOLVED=$(git rev-parse --verify "$TARGET" 2>/dev/null); then
  # Try as a remote branch
  if git rev-parse --verify "origin/$TARGET" >/dev/null 2>&1; then
    RESOLVED=$(git rev-parse --verify "origin/$TARGET")
    log "Resolved as remote branch origin/$TARGET"
  else
    err "Cannot resolve '$TARGET' to any known commit, tag, or branch."
  fi
fi

SHORT_RESOLVED=$(git rev-parse --short "$RESOLVED")
log "Resolved to commit: $SHORT_RESOLVED"

if [[ "$SHORT_RESOLVED" == "$PREVIOUS" ]]; then
  ok "Already on $SHORT_RESOLVED — nothing to do."
  exit 0
fi

# ── Stop services before switching ────────────────────────────────────────
log "Stopping services..."
$SUDO systemctl stop $SERVICE_DASHBOARD 2>/dev/null || warn "Dashboard service not running."
$SUDO systemctl stop $SERVICE_API       2>/dev/null || warn "API service not running."

# ── Checkout target ────────────────────────────────────────────────────────
log "Checking out $SHORT_RESOLVED..."
git checkout --detach "$RESOLVED" 2>&1 | tee -a "$BUILD_LOG" || {
  warn "Checkout failed — rolling back to $PREVIOUS..."
  git checkout --detach "$PREVIOUS"
  $SUDO systemctl start $SERVICE_API $SERVICE_DASHBOARD
  err "Switch aborted."
}

# ── Build Go control-plane ─────────────────────────────────────────────────
log "Building Go control-plane..."
if ! go build -o /usr/local/bin/klouds-server ./cmd/server/... 2>&1 | tee -a "$BUILD_LOG"; then
  warn "Go build failed — rolling back to $PREVIOUS..."
  git checkout --detach "$PREVIOUS"
  go build -o /usr/local/bin/klouds-server ./cmd/server/... 2>&1 | tee -a "$BUILD_LOG" || true
  $SUDO systemctl start $SERVICE_API $SERVICE_DASHBOARD
  err "Go build failed. Rolled back to $PREVIOUS."
fi
ok "Go build successful."

# ── Build SvelteKit dashboard (if changed) ─────────────────────────────────
CHANGED_DASHBOARD=$(git diff --name-only "$PREVIOUS" "$RESOLVED" -- dashboard/ 2>/dev/null | wc -l || echo 0)

if [[ "$CHANGED_DASHBOARD" -gt 0 ]]; then
  log "Dashboard changed ($CHANGED_DASHBOARD files) — rebuilding..."
  cd "$DASHBOARD_DIR"
  npm ci --prefer-offline 2>&1 | tee -a "$BUILD_LOG"
  npm run build 2>&1 | tee -a "$BUILD_LOG" || {
    warn "Dashboard build failed — rolling back to $PREVIOUS..."
    cd "$REPO_DIR"
    git checkout --detach "$PREVIOUS"
    go build -o /usr/local/bin/klouds-server ./cmd/server/... 2>&1 | tee -a "$BUILD_LOG" || true
    cd "$DASHBOARD_DIR" && npm ci --prefer-offline && npm run build 2>&1 | tee -a "$BUILD_LOG" || true
    $SUDO systemctl start $SERVICE_API $SERVICE_DASHBOARD
    err "Dashboard build failed. Rolled back to $PREVIOUS."
  }
  cd "$REPO_DIR"
  ok "Dashboard build successful."
else
  log "No dashboard changes detected — skipping dashboard rebuild."
fi

# ── Start services ─────────────────────────────────────────────────────────
log "Starting services..."
$SUDO systemctl start $SERVICE_API
sleep 2
$SUDO systemctl start $SERVICE_DASHBOARD

# ── Verify ────────────────────────────────────────────────────────────────
sleep 3
API_STATUS=$(systemctl is-active $SERVICE_API 2>/dev/null || echo "failed")
DASH_STATUS=$(systemctl is-active $SERVICE_DASHBOARD 2>/dev/null || echo "failed")

if [[ "$API_STATUS" != "active" || "$DASH_STATUS" != "active" ]]; then
  warn "One or more services failed to start after switch."
  warn "  $SERVICE_API:        $API_STATUS"
  warn "  $SERVICE_DASHBOARD:  $DASH_STATUS"
  warn "Rolling back to $PREVIOUS..."

  git checkout --detach "$PREVIOUS"
  go build -o /usr/local/bin/klouds-server ./cmd/server/... 2>&1 | tee -a "$BUILD_LOG" || true
  $SUDO systemctl restart $SERVICE_API $SERVICE_DASHBOARD
  err "Rollback complete. Now running: $PREVIOUS"
fi

# ── Done ───────────────────────────────────────────────────────────────────
NEW_HEAD=$(git rev-parse --short HEAD)
echo ""
ok "Switch complete!"
echo -e "  ${BOLD}Previous:${NC} $PREVIOUS"
echo -e "  ${BOLD}Now:${NC}      ${GREEN}$NEW_HEAD${NC}"
echo -e "  Build log: $BUILD_LOG"
