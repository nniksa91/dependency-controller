# Operator security guidance

Technical controls for installing and operating **dependency-controller**. This is not legal advice.

## Threat model (short)

The controller can **read** referenced objects and **update/patch** scalable dependents (`Deployment` / `StatefulSet` / `ReplicaSet` replicas). Compromise or overly broad RBAC expands blast radius to any object the service account can mutate.

**Namespace trust boundary:** refs are resolved in the same namespace as the `Dependency` CR. A principal who can create Dependency CRs in a namespace can point at any object name in that namespace the controller SA can read/mutate. Do not grant humans `dependency-editor-role` in namespaces they should not control.

## Secure install checklist

1. Install from a **pinned image digest** (or immutable tag you built and scanned), not `:latest` in production.
2. Prefer the Kustomize path (`make deploy`) so metrics authn/authz RBAC and HTTPS metrics patches stay aligned.
3. Confirm the Pod runs as non-root with dropped capabilities and `RuntimeDefault` seccomp (defaults in `config/manager` and `.helm/deployment.yaml`).
4. Keep the manager ServiceAccount's ClusterRole **least privilege**. Built-in rules cover only pods, apps workloads, jobs, and the `Dependency` CRD — **no** `resources: ["*"]` / `apiGroups: ["*"]` / `verbs: ["*"]`.
5. Enable leader election in multi-replica installs (`--leader-elect`).
6. Do **not** bind `cluster-admin` (or any wildcard ClusterRole) to the controller ServiceAccount.

## RBAC model (composability)

| Artifact | Audience | Bound to controller SA? |
|----------|----------|-------------------------|
| `manager-role` (`config/rbac/role.yaml`, `.helm/Role.yaml`) | Controller only | Yes (`manager-rolebinding`) |
| `leader-election-role` | Controller (namespaced) | Yes |
| `metrics-auth-role` | Controller (TokenReview / SubjectAccessReview create) | Yes when metrics auth enabled |
| `metrics-reader` | Prometheus / scrapers | **No** — bind scrapers separately |
| `dependency-editor-role` / `dependency-viewer-role` | Humans / CI | **No** — never bind to controller SA |
| `custom_dependency_reader_role.yaml` | Optional custom GVKs | Apply + bind (or merge rules) after editing placeholders |

Generated rules come from `+kubebuilder:rbac` markers in `internal/controller/dependency_controller.go` (`make manifests`). Keep `.helm/Role.yaml` in sync with `config/rbac/role.yaml` (helm uses different resource names).

### Why `update` on apps workloads?

`internal/gate` uses `client.Update` to set `spec.replicas` and the `dependency-controller/original-replicas` annotation. `update` is therefore required today. Switching to `patch`-only would need a gate rewrite; until then both `update` and `patch` stay on the ClusterRole.

### Dependency CR verbs

Controller: `get` / `list` / `watch` / `update` / `patch` on `dependencies`, plus status `get` / `update` / `patch`. **No** `create` / `delete` on the CR for the manager SA (humans use editor role).

### Custom resources (do not open everything)

Built-in ClusterRole rules do **not** grant access to arbitrary CRDs.

If a `Dependency` references a custom kind, add **only** the verbs you need — start from [`config/rbac/custom_dependency_reader_role.yaml`](../config/rbac/custom_dependency_reader_role.yaml):

| Role of the CR in the edge | Minimum verbs |
|----------------------------|---------------|
| Dependency (readiness only) | `get`, `list`, `watch` |
| Dependent that is scaled | also `update` / `patch` on that resource |

Do **not** grant `create`, `delete`, or wildcards unless you have a documented exception.

### ClusterRole vs namespaced Role

Default install uses a **ClusterRole** so `list`/`watch` work across namespaces (`make deploy` / multi-namespace Dependency CRs). That is a wider read surface than a namespaced `Role`.

If you run the operator for **one namespace only**:

1. Change `manager-role` / helm `dependency-controller-role` from `ClusterRole` to `Role` in that namespace.
2. Change the binding from `ClusterRoleBinding` to `RoleBinding`.
3. Deploy the manager into that namespace and create Dependency CRs only there.

Keep the ClusterRole for the default multi-namespace path; do not switch defaults without a migration plan.

## Zero-trust checklist

| Control | In-repo default | Notes |
|---------|-----------------|-------|
| Non-root UID | Yes (`runAsNonRoot`, distroless `65532`) | |
| Read-only root filesystem | Yes | |
| Drop all capabilities | Yes (`capabilities.drop: [ALL]`) | |
| `allowPrivilegeEscalation: false` | Yes | |
| seccomp `RuntimeDefault` | Yes | |
| SA token automount | `true` on controller SA/Pod only | Set `false` on any other workloads you co-install |
| No cluster-admin for controller SA | Documented / enforced by manifests | Review bindings in your cluster |
| Metrics off by default (helm); HTTPS+auth when enabled (Kustomize) | Yes | See below |
| NetworkPolicy | Optional (`config/network-policy/`) | Disabled by default — enable after tuning API CIDR / DNS |
| Image digest pinning | Operator responsibility | Prefer digest over floating tags |
| Admission / PSS | Cluster policy | Prefer `restricted` Pod Security for `dependency-system` |

### Enable optional NetworkPolicy

```sh
# Option A: uncomment `- ../network-policy` in config/default/kustomization.yaml, then:
make deploy IMG=<registry>/dependency-controller@sha256:<digest>

# Option B: build the component alone (set namespace to match your install):
kubectl apply -k config/network-policy
```

The sample policy allows ingress on `:8081` / `:8443` and egress to kube-system DNS plus TCP `443`/`6443`. Replace open API egress with your apiserver CIDR when known. CNIs without NetworkPolicy support will ignore the object.

## Metrics and probes

| Endpoint | Default | Notes |
|----------|---------|-------|
| Metrics | Disabled (`--metrics-bind-address=0`) unless enabled via Kustomize patch | When enabled, defaults to HTTPS + authn/authz (`--metrics-secure=true`) |
| Health probes | `:8081` (`/healthz`, `/readyz`) | No sensitive data; keep cluster-internal |

Metrics RBAC is narrow: `create` on `tokenreviews` and `subjectaccessreviews` only. Scrapers need a separate binding to `metrics-reader` (`get` on `/metrics`).

If you scrape metrics with Prometheus, replace `insecureSkipVerify: true` in `config/prometheus/monitor.yaml` with real TLS material for production.

## Secrets handling

This operator does **not** mount or process application Secrets. Support tickets and bug reports should redact Secret values and tokens (see [SUPPORT.md](../SUPPORT.md)).

## Image and supply chain

- Runtime image: `gcr.io/distroless/static:nonroot` (non-root UID `65532`).
- Rebuild with `make docker-build` after dependency changes; CI runs tests/lint and Dependabot watches `gomod` + Actions.
- CI runs `govulncheck` on every PR (currently informational / soft-fail while the project targets Go 1.22).
- Before a release, run `govulncheck ./...` locally and plan a Go toolchain bump when stdlib findings require it.
- Production: pin by digest, e.g. `IMG=registry.example.com/dependency-controller@sha256:…`.

## Gaps that need cluster / org policy (outside this repo)

These cannot be fully enforced by the operator manifests alone:

- Pod Security Admission / Kyverno / OPA constraints on the install namespace
- NetworkPolicy CNI enforcement and apiserver CIDR allowlists
- Image signature verification (cosign) and admission webhooks
- Cluster-wide RBAC reviews (no unexpected bindings to the controller SA)
- Multi-tenant isolation if untrusted users can create Dependency CRs

## Reporting issues

Private disclosure: [SECURITY.md](../SECURITY.md).
