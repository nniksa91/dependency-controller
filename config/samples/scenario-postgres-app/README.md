# Scenario: Postgres + app — scale gate instead of probe delays

Product thesis demo: Compose-style `depends_on` so an app that needs a real database does not start early, CrashLoop, or rely on guessed `initialDelaySeconds` / failure thresholds on the **app**.

| Mode | What happens |
|------|----------------|
| Without the Dependency gate | `demo-pg-app` schedules while Postgres is still initializing → `pg_isready` fails → process exits 1 → CrashLoopBackOff |
| With the Dependency CR | Controller keeps `demo-pg-app` at replicas=0 until `demo-postgres` has AvailableReplicas ≥ 1 → app starts once the DB accepts connections |

Design choices:

- **Postgres (dependency):** `readinessProbe` with `pg_isready` so Kubernetes reports AvailableReplicas for `serviceHealthy`.
- **App (dependent):** no liveness/readiness probes and no `initialDelaySeconds` waiting for the DB. Ordering is the Dependency CR scale gate.

Unlike [`scenario-app-waits-for-db`](../scenario-app-waits-for-db/) (nginx + artificial `sleep 45`), this uses PostgreSQL (`postgres:16-alpine`) and an app whose startup actually requires the DB.

## Manifests

| File | Resource |
|------|----------|
| `postgres-deployment.yaml` | `demo-postgres` — Postgres 16, `pg_isready` readiness |
| `postgres-service.yaml` | ClusterIP `demo-postgres:5432` |
| `app-deployment.yaml` | `demo-pg-app` — connect once then stay up; no probes |
| `dependency.yaml` | `serviceHealthy`: postgres Deployment → app Deployment |

Namespace: `default`.

## Prerequisites

- Cluster with dependency-controller running and CRD installed
- `postgres:16-alpine` pullable (or already cached) on the node

## Run

```sh
kubectl apply -k config/samples/scenario-postgres-app

# Watch scale-gate story (app stays 0 until DB Available)
watch -n1 'kubectl get deploy demo-postgres demo-pg-app; echo; kubectl get dependency postgres-app -o wide; echo; kubectl get pods -l demo.dependency-controller/scenario=postgres-app'
```

Automated asserts (app manifest has no probes; app stays 0 until DB available; then starts without CrashLoop):

```sh
./hack/test-postgres-app-dependency.sh
```

Optional educational contrast (scale the app up while the DB is down to see CrashLoop):

```sh
# After apply, while demo-postgres is NOT ready yet:
kubectl scale deploy/demo-pg-app --replicas=1
kubectl get pods -l app=demo-pg-app -w
# Restore gate behavior by re-applying the Dependency (or wait for reconcile after DB is up)
```

Or set `CONTRAST=1` when running the test script.

## Expected timeline (with CR)

| Phase | `demo-postgres` | `demo-pg-app` | Notes |
|-------|-----------------|---------------|-------|
| Early | Pulling / init; AvailableReplicas=0 | Scaled to 0 | No app pods → no CrashLoop |
| DB ready | `pg_isready` OK; Available≥1 | Scaled to 1 | Single successful start; app has no probes |
| Steady | Healthy | Running, restart=0 | Ordering was the gate, not `initialDelaySeconds` |

## Cleanup

```sh
kubectl delete -k config/samples/scenario-postgres-app --ignore-not-found
```
