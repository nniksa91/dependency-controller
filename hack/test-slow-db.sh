#!/usr/bin/env bash
# Live scenario: app waits for slow DB under the dependency-controller.
# Requires: kubectl context with CRD + controller; nginx image available on nodes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCENARIO="${ROOT}/config/samples/scenario-app-waits-for-db"
NS="${NS:-default}"
DB=demo-db
APP=demo-app
DEP=app-waits-for-db

# Init sleep in db-deployment.yaml is 45s; allow margin for scheduling/reconcile.
SCALE_DOWN_DEADLINE_S="${SCALE_DOWN_DEADLINE_S:-30}"
SCALE_UP_DEADLINE_S="${SCALE_UP_DEADLINE_S:-90}"
POLL_S="${POLL_S:-2}"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

replicas() {
  local name=$1 field=$2
  kubectl -n "$NS" get deploy "$name" -o "jsonpath={.${field}}" 2>/dev/null || echo "?"
}

status_line() {
  local db_spec db_avail app_spec app_avail ready scaled reason
  db_spec=$(replicas "$DB" "spec.replicas")
  db_avail=$(replicas "$DB" "status.availableReplicas")
  [[ -z "$db_avail" ]] && db_avail=0
  app_spec=$(replicas "$APP" "spec.replicas")
  app_avail=$(replicas "$APP" "status.availableReplicas")
  [[ -z "$app_avail" ]] && app_avail=0
  ready=$(kubectl -n "$NS" get dependency "$DEP" -o jsonpath='{.status.dependencyReady}' 2>/dev/null || echo "?")
  scaled=$(kubectl -n "$NS" get dependency "$DEP" -o jsonpath='{.status.dependentScaledDown}' 2>/dev/null || echo "?")
  reason=$(kubectl -n "$NS" get dependency "$DEP" -o jsonpath='{.status.reason}' 2>/dev/null || echo "?")
  printf 'db spec=%s avail=%s | app spec=%s avail=%s | dep ready=%s scaledDown=%s reason=%s\n' \
    "$db_spec" "$db_avail" "$app_spec" "$app_avail" "$ready" "$scaled" "$reason"
}

wait_until() {
  local desc=$1 deadline=$2
  shift 2
  local start=$SECONDS
  while (( SECONDS - start < deadline )); do
    if "$@"; then
      log "OK: $desc ($(status_line))"
      return 0
    fi
    log "… $(status_line)"
    sleep "$POLL_S"
  done
  log "FAIL: timed out waiting for: $desc"
  status_line
  return 1
}

app_scaled_to_zero() {
  local spec avail
  spec=$(replicas "$APP" "spec.replicas")
  avail=$(replicas "$APP" "status.availableReplicas")
  [[ -z "$avail" ]] && avail=0
  [[ "$spec" == "0" && "$avail" == "0" ]]
}

db_available() {
  local avail
  avail=$(replicas "$DB" "status.availableReplicas")
  [[ -n "$avail" && "$avail" != "0" && "$avail" != "?" ]]
}

app_scaled_up() {
  local spec avail
  spec=$(replicas "$APP" "spec.replicas")
  avail=$(replicas "$APP" "status.availableReplicas")
  [[ -z "$avail" ]] && avail=0
  [[ "$spec" != "0" && "$spec" != "?" && "$avail" != "0" ]]
}

cleanup() {
  log "Cleaning up scenario resources"
  kubectl delete -k "$SCENARIO" --ignore-not-found --wait=false >/dev/null 2>&1 || true
}

log "Context: $(kubectl config current-context)"
log "Applying scenario from $SCENARIO"
kubectl delete -k "$SCENARIO" --ignore-not-found --wait=true >/dev/null 2>&1 || true
# Small pause so old pods terminate before re-apply (cleaner timeline).
sleep 2
kubectl apply -k "$SCENARIO"

log "Phase 1: expect app scaled to 0 while DB init sleeps"
wait_until "app scaled to 0 (DB not healthy yet)" "$SCALE_DOWN_DEADLINE_S" app_scaled_to_zero

log "Phase 2: wait for DB AvailableReplicas > 0 (after ~45s init)"
wait_until "DB available" "$SCALE_UP_DEADLINE_S" db_available

log "Phase 3: expect app scaled back up"
wait_until "app scaled up after DB ready" 45 app_scaled_up

log "PASS — replica timeline matched expected gate behavior"
status_line
echo
kubectl -n "$NS" get deploy "$DB" "$APP" -o wide
kubectl -n "$NS" get dependency "$DEP" -o wide
kubectl -n "$NS" get pods -l 'demo.dependency-controller/scenario=app-waits-for-db' -o wide 2>/dev/null || \
  kubectl -n "$NS" get pods -l 'app in (demo-db,demo-app)' -o wide

if [[ "${KEEP:-0}" != "1" ]]; then
  cleanup
else
  log "KEEP=1 set; leaving resources in place"
fi
