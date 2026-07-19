# CRD reference — `Dependency`

| Field | Value |
|-------|--------|
| API group | `core.example.com` |
| Version | `v1` |
| Kind | `Dependency` |
| Scope | Namespaced |

`core.example.com` is this project’s CRD API group name (from the Kubebuilder scaffold). It is not related to the public `example.com` website. Sample YAML that uses `db.example.com` is a **placeholder** for a custom dependency CRD you own.

## Support matrix

| Role | Kinds | Effect |
|------|-------|--------|
| Dependency | `Deployment`, `StatefulSet`, `ReplicaSet`, `Pod`, `Job`, custom resources | Readiness via `condition` / optional `readyWhen` |
| Dependent (scalable) | `Deployment`, `StatefulSet`, `ReplicaSet` | Scale to `0` / restore replicas |
| Dependent (other) | `Pod`, `Job`, most CRs | Status only (`DependentNotScalable`) |

Refs resolve in the **Dependency CR’s namespace**. `ObjectRef` has no `namespace` field.

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
  readyWhen:                  # optional; for custom-resource dependencies
    jsonPath: "{.status.phase}"
    value: "Ready"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.dependency` | ObjectRef | yes | Object that must satisfy `condition` |
| `spec.dependent` | ObjectRef | yes | Object to gate (scaled only if scalable) |
| `spec.condition` | string | no | Default `serviceHealthy` |
| `spec.desiredReplicas` | int32 | no | Scale-up target override |
| `spec.readyWhen` | object | no | JSONPath match for CR readiness when no Ready condition |

### ObjectRef

| Field | Description |
|-------|-------------|
| `apiVersion` | e.g. `apps/v1`, `v1`, or your CRD group/version |
| `kind` | e.g. `Deployment`, `StatefulSet`, `Pod`, `Job`, or a CR kind |
| `name` | Object name in the Dependency’s namespace |

### Conditions

| Value | Typical use |
|-------|-------------|
| `serviceStarted` | Object exists and is not terminating (workloads: has started replicas when applicable) |
| `serviceHealthy` | Workloads: available/ready replicas; Pod: Ready; CRs: Ready condition, else `readyWhen`, else exists |
| `serviceCompleted` | Job Complete or Pod Succeeded |

## Status

| Field | Description |
|-------|-------------|
| `dependencyReady` | Dependency satisfied the condition |
| `dependentScaledDown` | Scalable dependent currently at 0 replicas |
| `condition` | Effective condition used |
| `reason` | Short CamelCase reason (see below) |
| `message` | Human-readable detail |
| `observedGeneration` | Last processed `.metadata.generation` |

### Common `status.reason` values

| Reason | Meaning |
|--------|---------|
| `Available` / `ReadyReplicas` / `PodReady` | Dependency healthy |
| `NotAvailable` / `PodNotReady` / `JobNotComplete` | Dependency not yet ready |
| `DependencyMissing` / `DependentMissing` | Referenced object not found |
| `ScaledDown` / `ScaledUp` / `AlreadyAtTarget` | Gate action on a scalable dependent |
| `DependentNotScalable` | Dependent kind is not scaled |
| `JSONPathMatch` / `JSONPathMismatch` | `readyWhen` evaluation |
| `ReadyCondition` | CR `status.conditions` Ready=True |

## Samples

| File | Scenario |
|------|----------|
| `config/samples/core_v1_dependency.yaml` | Deploy → Deploy |
| `config/samples/core_v1_dependency_statefulset.yaml` | Deploy waits for StatefulSet |
| `config/samples/core_v1_dependency_job.yaml` | Deploy waits for Job (`serviceCompleted`) |
| `config/samples/core_v1_dependency_cr.yaml` | Deploy waits for CR + `readyWhen` (placeholder GVK) |
| [`config/samples/scenario-postgres-app/`](../config/samples/scenario-postgres-app/) | Postgres + app (no app probes); scale gate avoids CrashLoop and static probe delays |
| [`config/samples/scenario-app-waits-for-db/`](../config/samples/scenario-app-waits-for-db/) | Synthetic slow DB (nginx + init sleep) for a quick timeline demo |
