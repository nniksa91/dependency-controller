# CRD reference — `Dependency`

| Field | Value |
|-------|--------|
| API group | `core.example.com` |
| Version | `v1` |
| Kind | `Dependency` |
| Scope | Namespaced |

## Spec

```yaml
apiVersion: core.example.com/v1
kind: Dependency
metadata:
  name: app-waits-for-db
  namespace: default
spec:
  condition: serviceHealthy   # serviceStarted | serviceHealthy | serviceCompleted
  dependency:
    apiVersion: apps/v1
    kind: StatefulSet
    name: db
  dependent:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  desiredReplicas: 3          # optional
  readyWhen:                  # optional; for CRs
    jsonPath: "{.status.phase}"
    value: "Ready"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.dependency` | ObjectRef | yes | Object that must satisfy `condition` |
| `spec.dependent` | ObjectRef | yes | Object to gate (scale if scalable) |
| `spec.condition` | string | no | Default `serviceHealthy` |
| `spec.desiredReplicas` | int32 | no | Scale-up target override |
| `spec.readyWhen` | object | no | JSONPath match for CR readiness |

### ObjectRef

| Field | Description |
|-------|-------------|
| `apiVersion` | e.g. `apps/v1`, `v1`, `db.example.com/v1` |
| `kind` | e.g. `Deployment`, `StatefulSet`, `Pod`, `Job`, `Database` |
| `name` | Object name in the Dependency’s namespace |

## Status

| Field | Description |
|-------|-------------|
| `dependencyReady` | Dependency satisfied the condition |
| `dependentScaledDown` | Scalable dependent at 0 replicas |
| `condition` | Effective condition used |
| `reason` | Short reason (`NotAvailable`, `DependentNotScalable`, …) |
| `message` | Human-readable detail |
| `observedGeneration` | Last processed generation |

## Samples

| File | Scenario |
|------|----------|
| `config/samples/core_v1_dependency.yaml` | Deploy → Deploy |
| `config/samples/core_v1_dependency_statefulset.yaml` | Deploy → StatefulSet |
| `config/samples/core_v1_dependency_job.yaml` | Deploy waits for Job (`serviceCompleted`) |
| `config/samples/core_v1_dependency_cr.yaml` | Deploy → CR + `readyWhen` |
| [`config/samples/scenario-postgres-app/`](../config/samples/scenario-postgres-app/) | **Real stack** — Postgres + app (no app probes); scale-to-zero avoids CrashLoop and static probe delays |
| [`config/samples/scenario-app-waits-for-db/`](../config/samples/scenario-app-waits-for-db/) | Synthetic slow DB (nginx + init sleep) for a quick timeline demo |
