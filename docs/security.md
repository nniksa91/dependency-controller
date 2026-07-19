# Operator security guidance

Technical controls for installing and operating **dependency-controller**. This is not legal advice.

## Threat model (short)

The controller can **read** referenced objects and **update/patch** scalable dependents (`Deployment` / `StatefulSet` / `ReplicaSet` replicas). Compromise or overly broad RBAC expands blast radius to any object the service account can mutate.

## Secure install checklist

1. Install from a **pinned image digest** (or immutable tag you built and scanned), not `:latest` in production.
2. Prefer the Kustomize path (`make deploy`) so metrics authn/authz RBAC and HTTPS metrics patches stay aligned.
3. Confirm the Pod runs as non-root with dropped capabilities and `RuntimeDefault` seccomp (defaults in `config/manager` and `.helm/deployment.yaml`).
4. Keep the manager ServiceAccount's ClusterRole **least privilege**. Built-in rules cover only pods, apps workloads, jobs, and the `Dependency` CRD.
5. Enable leader election in multi-replica installs (`--leader-elect`).

## RBAC and custom resources

Built-in ClusterRole rules do **not** grant access to arbitrary CRDs.

If a `Dependency` references a custom kind, add **only** the verbs you need:

| Role of the CR in the edge | Minimum verbs |
|----------------------------|---------------|
| Dependency (readiness only) | `get`, `list`, `watch` |
| Dependent that is scaled | also `update` / `patch` on that resource |

Do **not** grant `create`, `delete`, or cluster-scoped wildcards (`resources: ["*"]`) unless you have a documented exception.

## Metrics and probes

| Endpoint | Default | Notes |
|----------|---------|-------|
| Metrics | Disabled (`--metrics-bind-address=0`) unless enabled via Kustomize patch | When enabled, defaults to HTTPS + authn/authz (`--metrics-secure=true`) |
| Health probes | `:8081` (`/healthz`, `/readyz`) | No sensitive data; keep cluster-internal |

If you scrape metrics with Prometheus, replace `insecureSkipVerify: true` in `config/prometheus/monitor.yaml` with real TLS material for production.

## Secrets handling

This operator does **not** mount or process application Secrets. Support tickets and bug reports should redact Secret values and tokens (see [SUPPORT.md](../SUPPORT.md)).

## Image and supply chain

- Runtime image: `gcr.io/distroless/static:nonroot` (non-root UID `65532`).
- Rebuild with `make docker-build` after dependency changes; CI runs tests/lint and Dependabot watches `gomod` + Actions.
- CI runs `govulncheck` on every PR (currently informational / soft-fail while the project targets Go 1.22).
- Before a release, run `govulncheck ./...` locally and plan a Go toolchain bump when stdlib findings require it.

## Reporting issues

Private disclosure: [SECURITY.md](../SECURITY.md).
