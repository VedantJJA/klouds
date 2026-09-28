#!/usr/bin/env bash
# ==============================================================================
#  switch-paas.sh - Switch between current PaaS (klouds) and old PaaS (kloudsPanel)
#
#  Usage:
#    ./switch-paas.sh old       -> Switch to old PaaS (kloudsPanel / docker-compose)
#    ./switch-paas.sh new       -> Switch to current PaaS (klouds / Caddy + systemd)
#    ./switch-paas.sh status    -> Show which PaaS is currently active
# ==============================================================================

set -uo pipefail

OLD_PAAS_DIR="/home/ubuntu/paas"
OLD_COMPOSE_FILE="${OLD_PAAS_DIR}/paas/deploy/compose/compose.platform.yaml"
OLD_ENV_FILE="${OLD_PAAS_DIR}/paas/deploy/compose/.env"

CURRENT_SERVICES=("caddy.service" "klouds.service" "klouds-dashboard.service")
CURRENT_POSTGRES="klouds-postgres"

show_help() {
    echo "Usage: $0 [old|new|status]"
    echo ""
    echo "Options:"
    echo "  old, panel, kloudspanel     Switch to old PaaS (kloudsPanel with Traefik)"
    echo "  new, current, klouds        Switch to current PaaS (klouds with Caddy & Go)"
    echo "  status                      Show which PaaS is currently running"
    echo ""
}

get_status() {
    local caddy_active=false
    local old_active=false

    if systemctl is-active --quiet caddy.service 2>/dev/null; then
        caddy_active=true
    fi

    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q 'klouds-traefik\|klouds-api\|klouds-web'; then
        old_active=true
    fi

    echo "=========================================="
    echo "          PaaS Platform Status            "
    echo "=========================================="
    if [ "$caddy_active" = true ] && [ "$old_active" = false ]; then
        echo "Active PaaS: CURRENT (klouds)"
        echo " - Caddy Router:       ACTIVE (ports 80, 443)"
        echo " - Klouds API Server:  ACTIVE (port 8080)"
        echo " - Klouds Dashboard:   ACTIVE (port 3000)"
        echo " - Postgres DB:        $(docker inspect -f '{{.State.Status}}' "$CURRENT_POSTGRES" 2>/dev/null || echo 'not found')"
    elif [ "$old_active" = true ] && [ "$caddy_active" = false ]; then
        echo "Active PaaS: OLD (kloudsPanel)"
        echo " - Traefik Proxy:      ACTIVE (ports 80, 443)"
        echo " - Old Panel Stack:    RUNNING (docker-compose)"
    elif [ "$caddy_active" = true ] && [ "$old_active" = true ]; then
        echo "WARNING: PORT CONFLICT DETECTED! Both platforms have running components."
    else
        echo "Active PaaS: NONE (both platforms are stopped)"
    fi
    echo "=========================================="
}

switch_to_old() {
    echo "==> Stopping current PaaS (klouds services)..."
    sudo systemctl stop "${CURRENT_SERVICES[@]}" 2>/dev/null || true
    echo "    Current PaaS services stopped."

    echo "==> Starting old PaaS (kloudsPanel)..."
    if [ ! -f "$OLD_COMPOSE_FILE" ]; then
        echo "ERROR: Old PaaS compose file not found at $OLD_COMPOSE_FILE"
        exit 1
    fi

    docker compose -f "$OLD_COMPOSE_FILE" --env-file "$OLD_ENV_FILE" up -d --remove-orphans
    echo "==> Old PaaS (kloudsPanel) started successfully."
    echo ""
    get_status
}

switch_to_new() {
    echo "==> Stopping old PaaS (kloudsPanel)..."
    if [ -f "$OLD_COMPOSE_FILE" ]; then
        docker compose -f "$OLD_COMPOSE_FILE" --env-file "$OLD_ENV_FILE" down 2>/dev/null || true
    fi
    # Ensure any remaining legacy containers are stopped
    docker stop klouds-web klouds-api klouds-traefik klouds-sablier 2>/dev/null || true
    echo "    Old PaaS stopped."

    echo "==> Starting current PaaS (klouds)..."
    # Ensure postgres container is running
    docker start "$CURRENT_POSTGRES" 2>/dev/null || true

    # Start systemd services
    sudo systemctl daemon-reload
    sudo systemctl start klouds.service klouds-dashboard.service caddy.service
    echo "==> Current PaaS (klouds) started successfully."
    echo ""
    get_status
}

TARGET="${1:-}"

case "$TARGET" in
    old|panel|kloudspanel)
        switch_to_old
        ;;
    new|current|klouds)
        switch_to_new
        ;;
    status|check)
        get_status
        ;;
    help|-h|--help)
        show_help
        ;;
    *)
        echo "Error: Unknown argument '$TARGET'"
        echo ""
        show_help
        exit 1
        ;;
esac
