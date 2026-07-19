# Scenario: app waits for slow DB

Demonstrates Compose-style `depends_on` with `serviceHealthy`: **demo-app** stays at 0 replicas while **demo-db** is still initializing (45s init sleep), then scales back up once the DB Deployment is available.

The controller replaces static probe delays on the dependent: `demo-app` has no liveness/readiness probes and no `initialDelaySeconds` guessing. The slow DB uses an init `sleep` plus a readiness probe so Kubernetes reports AvailableReplicas — that is the dependency signal, not an app wait strategy.

Without the controller, `demo-app` would become Ready within a few seconds while `demo-db` was still in its init container.

## Manifests

| File | Resource |
|------|----------|
| `db-deployment.yaml` | `demo-db` — nginx with `initContainers` `sleep 45` + readiness |
| `app-deployment.yaml` | `demo-app` — fast nginx, `replicas: 1`, no probes |
| `dependency.yaml` | `Dependency` CR linking db → app |

Namespace: `default`.

## Prerequisites

- Cluster with the dependency-controller running and CRD installed
- `nginx:latest` available (or pullable) on the node

## Manual steps

```sh
kubectl apply -k config/samples/scenario-app-waits-for-db

# Watch replicas and Dependency status
watch -n1 'kubectl get deploy demo-db demo-app; echo; kubectl get dependency app-waits-for-db -o wide'
```

Or one-shot timeline:

```sh
./hack/test-slow-db.sh
```

## Expected timeline

| Time (approx) | `demo-db` | `demo-app` | Dependency status |
|---------------|-----------|------------|-------------------|
| t≈0–5s | Pod starting / init sleeping; AvailableReplicas=0 | Scaled to 0 by controller | `dependencyReady=false`, `scaledDown=true` |
| t≈0–45s | Init container `sleep 45` still running | Remains 0 | Not ready |
| t≈45–50s | Main container Ready; AvailableReplicas≥1 | Scaled back to 1 | `dependencyReady=true`, `scaledDown=false` |

Key assertion: while the DB init sleeps, `demo-app` must show `replicas: 0` (or desired=0), not a running Ready pod.

## Cleanup

```sh
kubectl delete -k config/samples/scenario-app-waits-for-db --ignore-not-found
```
