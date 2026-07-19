#!/usr/bin/env bash
# Live scenario: real Postgres + dependent app under the dependency-controller.
# Asserts: app manifest has no liveness/readiness probes; app stays at replicas=0
# until Postgres is Available; then scales up without CrashLoopBackOff.
#
# Requires: kubectl context with CRD + controller; postgres:16-alpine pullable.
# Env:
#   KEEP=1       leave resources after PASS
#   CONTRAST=1   after scale-down assert, briefly force app=1 while DB not ready
#                and show restart/CrashLoop (educational; then wait for recovery)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCENARIO="${ROOT}/config/samples/scenario-postgres-app"
NS="${NS:-default}"
DB=demo-postgres
APP=demo-pg-app
DEP=postgres-app
LABEL='demo.dependency-controller/scenario=postgres-app'
APP_MANIFEST="${SCENARIO}/app-deployment.yaml"

# Image pull + Postgres init can take a while on first run.
SCALE_DOWN_DEADLINE_S="${SCALE_DOWN_DEADLINE_S:-45}"
DB_READY_DEADLINE_S="${DB_READY_DEADLINE_S:-180}"
SCALE_UP_DEADLINE_S="${SCALE_UP_DEADLINE_S:-60}"
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
  printf 'pg spec=%s avail=%s | app spec=%s avail=%s | dep ready=%s scaledDown=%s reason=%s\n' \
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

app_not_crashlooping() {
  # No app pod in CrashLoopBackOff; restart count on Running pods stays low.
  local bad
  bad=$(kubectl -n "$NS" get pods -l "app=${APP}" \
    -o jsonpath='{range .items[*]}{.status.phase}{" "}{.status.containerStatuses[0].state.waiting.reason}{" "}{.status.containerStatuses[0].restartCount}{"\n"}{end}' \
    2>/dev/null || true)
  if echo "$bad" | grep -q CrashLoopBackOff; then
    return 1
  fi
  # After scale-up, require at least one Running pod with restartCount <= 2
  # (allow a single race restart; gate should prevent a loop).
  local line phase reason restarts
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    phase=$(echo "$line" | awk '{print $1}')
    reason=$(echo "$line" | awk '{print $2}')
    restarts=$(echo "$line" | awk '{print $3}')
    [[ "$phase" == "Running" && "${restarts:-0}" -le 2 ]] && return 0
  done <<< "$bad"
  return 1
}

assert_app_manifest_has_no_probes() {
  if grep -E '^\s*(livenessProbe|readinessProbe):' "$APP_MANIFEST" >/dev/null; then
    log "FAIL: $APP_MANIFEST must not define livenessProbe/readinessProbe (product thesis)"
    return 1
  fi
  log "OK: app manifest has no liveness/readiness probes"
}

cleanup() {
  log "Cleaning up scenario resources"
  kubectl delete -k "$SCENARIO" --ignore-not-found --wait=false >/dev/null 2>&1 || true
}

contrast_without_gate() {
  # Educational: force app up while DB not ready → expect exit/restart pain.
  log "CONTRAST=1: forcing app replicas=1 while Postgres not ready (expect restarts)"
  kubectl -n "$NS" scale "deploy/${APP}" --replicas=1
  local i=0
  while (( i < 30 )); do
    kubectl -n "$NS" get pods -l "app=${APP}" -o wide 2>/dev/null || true
    if kubectl -n "$NS" get pods -l "app=${APP}" \
      -o jsonpath='{range .items[*]}{.status.containerStatuses[0].state.waiting.reason}{" "}{.status.containerStatuses[0].restartCount}{"\n"}{end}' \
      2>/dev/null | grep -qE 'CrashLoopBackOff|[1-9]'; then
      log "CONTRAST observed: app restarting / CrashLoop without healthy DB (this is the pain the gate avoids)"
      break
    fi
    sleep 2
    i=$((i + 1))
  done
  log "CONTRAST done — waiting for controller + Postgres to restore healthy path"
  # Re-apply Dependency so status/reconcile is fresh; controller should scale to 0
  # again if DB still not ready, then scale up when ready.
  kubectl apply -f "${SCENARIO}/dependency.yaml" >/dev/null
}

assert_app_manifest_has_no_probes

log "Context: $(kubectl config current-context)"
log "Applying scenario from $SCENARIO"
kubectl delete -k "$SCENARIO" --ignore-not-found --wait=true >/dev/null 2>&1 || true
# Remove leftover CR/name from the pre-rename scenario if still present.
kubectl delete dependency postgres-app-probes -n "$NS" --ignore-not-found --wait=false >/dev/null 2>&1 || true
sleep 2
kubectl apply -k "$SCENARIO"

log "Phase 1: expect app scaled to 0 while Postgres not Available"
wait_until "app scaled to 0 (Postgres not healthy yet)" "$SCALE_DOWN_DEADLINE_S" app_scaled_to_zero

if [[ "${CONTRAST:-0}" == "1" ]]; then
  # Only run contrast if DB still not available (otherwise pain window is gone).
  if ! db_available; then
    contrast_without_gate
  else
    log "CONTRAST skipped: Postgres already Available"
  fi
fi

log "Phase 2: wait for Postgres AvailableReplicas > 0"
wait_until "Postgres available" "$DB_READY_DEADLINE_S" db_available

log "Phase 3: expect app scaled up and not CrashLooping"
wait_until "app scaled up after Postgres ready" "$SCALE_UP_DEADLINE_S" app_scaled_up
wait_until "app not CrashLoopBackOff (restartCount low)" 45 app_not_crashlooping

# Live deploy must also lack probes (not only the file on disk).
if kubectl -n "$NS" get deploy "$APP" -o yaml | grep -E '^\s*(livenessProbe|readinessProbe):' >/dev/null; then
  log "FAIL: live deploy/${APP} still has probes"
  exit 1
fi
log "OK: live deploy/${APP} has no liveness/readiness probes"

log "PASS — scale gate ordered startup; app has no probes; no CrashLoop after DB ready"
status_line
echo
kubectl -n "$NS" get deploy "$DB" "$APP" -o wide
kubectl -n "$NS" get svc demo-postgres -o wide 2>/dev/null || true
kubectl -n "$NS" get dependency "$DEP" -o wide
kubectl -n "$NS" get pods -l "$LABEL" -o wide

if [[ "${KEEP:-0}" != "1" ]]; then
  cleanup
else
  log "KEEP=1 set; leaving resources in place"
fi
