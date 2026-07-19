# Operator security guidance

Technical controls for installing and operating **dependency-controller**. This is not legal advice.

## Threat model (short)

The controller can **read** referenced objects and **update/patch** scalable dependents (`Deployment` / `StatefulSet` / `ReplicaSet` replicas). Compromise or overly broad RBAC expands blast radius to any object the service account can mutate.

**Namespace trust boundary:** refs are resolved in the same namespace as the `Dependency` CR (`ObjectRef` has no namespace field). A principal who can create Dependency CRs in a namespace can point at any object name in that namespace the controller SA can read/mutate.

**Multi-tenant warning:** with the default **ClusterRole**, the controller SA can scale workloads **cluster-wide**. Anyone who can create a `Dependency` in *any* namespace the controller reconciles can therefore drive scale mutations on Deployments/StatefulSets/ReplicaSets the SA can touch — not only in their own namespace. Do **not** grant untrusted users `dependency-editor-role`, cluster-admin, or broad `edit` on `dependencies.core.example.com`. Prefer the namespaced [`dependency-creator-role`](../config/rbac/dependency_creator_role.yaml) plus admission policies (below), or run a **single-namespace** controller ([`config/rbac/namespaced/`](../config/rbac/namespaced/)).

## Secure install checklist

1. Install from a **pinned image digest** (or immutable tag you built and scanned), not `:latest` in production.
2. Prefer the Kustomize path (`make deploy`) so metrics authn/authz RBAC and HTTPS metrics patches stay aligned.
3. Confirm the Pod runs as non-root with dropped capabilities and `RuntimeDefault` seccomp (defaults in `config/manager` and `.helm/deployment.yaml`). The install namespace is labeled for PSA **restricted**.
4. Keep the manager ServiceAccount's ClusterRole **least privilege**. Built-in rules cover only pods, apps workloads, jobs, and the `Dependency` CRD — **no** `resources: ["*"]` / `apiGroups: ["*"]` / `verbs: ["*"]`.
5. Enable leader election in multi-replica installs (`--leader-elect`).
6. Do **not** bind `cluster-admin` (or any wildcard ClusterRole) to the controller ServiceAccount.
7. Optionally enable NetworkPolicy, VAP, and/or Kyverno packs after editing placeholders — see [Optional policy packs](#optional-policy-packs).

## RBAC model (composability)

| Artifact | Audience | Bound to controller SA? |
|----------|----------|-------------------------|
| `manager-role` (`config/rbac/role.yaml`, `.helm/Role.yaml`) | Controller only | Yes (`manager-rolebinding`) |
| `leader-election-role` | Controller (namespaced) | Yes |
| `metrics-auth-role` | Controller (TokenReview / SubjectAccessReview create) | Yes when metrics auth enabled |
| `metrics-reader` | Prometheus / scrapers | **No** — bind scrapers separately |
| `dependency-editor-role` / `dependency-viewer-role` | Humans / CI (cluster-scoped helpers) | **No** — never bind to controller SA |
| `dependency-creator-role` | Humans / CI (**namespaced** Role) | **No** — apply + bind per tenant NS |
| `custom_dependency_reader_role.yaml` | Optional custom GVKs | Apply + bind (or merge rules) after editing placeholders |
| `config/rbac/namespaced/` | Single-NS controller Role | Optional alternate to ClusterRole |

Generated rules come from `+kubebuilder:rbac` markers in `internal/controller/dependency_controller.go` (`make manifests`). Keep `.helm/Role.yaml` in sync with `config/rbac/role.yaml` (helm uses different resource names).

### Why `update` on apps workloads?

`internal/gate` uses `client.Update` to set `spec.replicas` and the `dependency-controller/original-replicas` annotation. `update` is therefore required today. Switching to `patch`-only would need a gate rewrite; until then both `update` and `patch` stay on the ClusterRole.

### Dependency CR verbs

Controller: `get` / `list` / `watch` / `update` / `patch` on `dependencies`, plus status `get` / `update` / `patch`. **No** `create` / `delete` on the CR for the manager SA (humans use editor/creator roles).

### Custom resources (do not open everything)

Built-in ClusterRole rules do **not** grant access to arbitrary CRDs.

If a `Dependency` references a custom kind, add **only** the verbs you need — start from [`config/rbac/custom_dependency_reader_role.yaml`](../config/rbac/custom_dependency_reader_role.yaml):

| Role of the CR in the edge | Minimum verbs |
|----------------------------|---------------|
| Dependency (readiness only) | `get`, `list`, `watch` |
| Dependent that is scaled | also `update` / `patch` on that resource |

Do **not** grant `create`, `delete`, or wildcards unless you have a documented exception.

### ClusterRole vs namespaced Role

Default install uses a **ClusterRole** so `list`/`watch` work across namespaces (`make deploy` / multi-namespace Dependency CRs). That is a wider read/mutate surface than a namespaced `Role`.

If you run the operator for **one namespace only**:

1. Use [`config/rbac/namespaced/`](../config/rbac/namespaced/) (Role + RoleBinding) instead of the ClusterRoleBinding — see that directory's README.
2. Deploy the manager into that namespace and create Dependency CRs only there.
3. Alternatively, convert helm `dependency-controller-role` from ClusterRole to Role and use a RoleBinding.

Keep the ClusterRole for the default multi-namespace path; do not switch defaults without a migration plan.

### Human creators (per namespace)

```sh
kubectl apply -f config/rbac/dependency_creator_role.yaml -n <tenant-ns>
kubectl create rolebinding dependency-creators \
  --role=dependency-creator-role \
  --group=dependency-creators \
  -n <tenant-ns>
```

Avoid granting random users cluster-admin or the broad `edit` ClusterRole solely to manage Dependency CRs.

## Zero-trust checklist

| Control | In-repo default | Notes |
|---------|-----------------|-------|
| Non-root UID | Yes (`runAsNonRoot`, `runAsUser: 65532`) | Distroless nonroot |
| Read-only root filesystem | Yes | |
| Drop all capabilities | Yes (`capabilities.drop: [ALL]`) | |
| `allowPrivilegeEscalation: false` | Yes | |
| seccomp `RuntimeDefault` | Yes | |
| PSA `restricted` on install NS | Yes (Kustomize Namespace labels) | `dependency-system` |
| SA token automount | `true` on controller SA/Pod only | Set `false` on any other workloads you co-install |
| No cluster-admin for controller SA | Documented / enforced by manifests | Review bindings periodically |
| Metrics off by default (helm); HTTPS+auth when enabled (Kustomize) | Yes | See below |
| NetworkPolicy | Optional (`config/network-policy/`) | Disabled by default — tighten API CIDR |
| Image digest pinning | Operator responsibility | `IMG=...@sha256:…` |
| Admission (VAP / Kyverno) | Optional (`config/policy/`) | Disabled by default |
| Cosign verify | Optional Kyverno EXAMPLE | Needs your public key |

### Pod Security Admission (restricted)

`config/manager/manager.yaml` labels the controller Namespace with:

- `pod-security.kubernetes.io/enforce=restricted`
- `audit` / `warn` = `restricted`

After `make deploy`, verify:

```sh
kubectl get ns dependency-system --show-labels | grep pod-security
kubectl -n dependency-system get deploy dependency-controller-manager -o yaml | grep -A20 securityContext
```

Helm installs into whatever namespace you choose (default `default` in `.helm/deployment.yaml`) and does **not** create a Namespace object. Label that namespace yourself if you want PSA restricted:

```sh
kubectl label ns <ns> \
  pod-security.kubernetes.io/enforce=restricted \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted \
  --overwrite
```

### Enable optional NetworkPolicy

```sh
# Option A: uncomment `- ../network-policy` in config/default/kustomization.yaml, then:
make deploy IMG=<registry>/dependency-controller@sha256:<digest>

# Option B: build the component alone (set namespace to match your install):
kubectl apply -k config/network-policy
```

The sample policy allows ingress on `:8081` / `:8443` and egress to kube-system DNS plus TCP `443`/`6443`. The API egress rule has **no** `to:` selector (open to any destination on those ports — treat like `0.0.0.0/0`). Before production, replace it with apiserver `ipBlock` CIDRs using the commented example in [`config/network-policy/controller-manager.yaml`](../config/network-policy/controller-manager.yaml):

```sh
kubectl -n default get endpoints kubernetes -o wide
kubectl -n default get svc kubernetes -o jsonpath='{.spec.clusterIP}{"\n"}'
```

CNIs without NetworkPolicy support will ignore the object.

### Digest pinning

```sh
# Preferred: digest reference
make deploy IMG=ghcr.io/nniksa91/dependency-controller@sha256:<digest>

# Resolve a tag to a digest first (example)
crane digest ghcr.io/nniksa91/dependency-controller:<tag>
# or: docker buildx imagetools inspect <image>:<tag>
```

`make deploy` runs `kustomize edit set image controller=${IMG}` under `config/manager`. You can also set an `images:` digest transformer manually — see comments in [`config/manager/kustomization.yaml`](../config/manager/kustomization.yaml).

## Optional policy packs

Artifacts under [`config/policy/`](../config/policy/) are **optional** and **off by default** so `make deploy` never requires Kyverno.

| Pack | Enable | Edit first |
|------|--------|------------|
| ValidatingAdmissionPolicy | `kubectl apply -k config/policy/vap` | Allowed groups; controller SA prefix; starts in `Audit` |
| Kyverno creators + cosign EXAMPLE | `kubectl apply -k config/policy/kyverno` | Groups; cosign public key; starts in `Audit` |
| Wire into default kustomize | Uncomment `../policy/vap` / `../policy/kyverno` in `config/default/kustomization.yaml` | Same — Kyverno CRDs must exist |

Details and placeholders: [`config/policy/README.md`](../config/policy/README.md).

Out-of-repo steps you still own:

1. Install Kyverno (if using that pack).
2. Generate/distribute Cosign keys (or configure keyless OIDC) and replace the EXAMPLE public key.
3. Map IdP groups to the allow-lists and to `dependency-creator-role` RoleBindings.
4. Flip Kyverno / VAP from Audit → Enforce after dry-runs.

## Binding review checklist

Run periodically (e.g. quarterly) and after any RBAC change:

1. List bindings for the controller ServiceAccount:
   ```sh
   kubectl get clusterrolebinding,rolebinding -A -o json \
     | jq -r '.items[] | select(.subjects[]? | .name=="dependency-controller-manager" or .name=="dependency-controller-sa") | "\(.kind)/\(.metadata.namespace // "cluster")/\(.metadata.name) -> \(.roleRef.kind)/\(.roleRef.name)"'
   ```
2. Confirm **no** binding to `cluster-admin`, `admin`, `edit`, or any Role/ClusterRole with wildcards (`resources: ["*"]`, `verbs: ["*"]`, `apiGroups: ["*"]`).
3. Confirm human helper roles (`dependency-editor-role`, `dependency-viewer-role`, `dependency-creator-role`) are **not** bound to the controller SA.
4. Confirm scrapers use `metrics-reader` only (not the manager ClusterRole).
5. For multi-tenant clusters: re-check who can `create` on `dependencies.core.example.com` in each namespace (`kubectl auth can-i create dependencies -n <ns> --as=...`).
6. Record exceptions (who approved, why, expiry, compensating control).

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
- Optional: Kyverno `verifyImages` EXAMPLE under [`config/policy/kyverno/verify-images-example.yaml`](../config/policy/kyverno/verify-images-example.yaml).

## Gaps that need cluster / org action (outside default manifests)

In-repo optional packs cover the *artifacts*; you still must apply and operate them:

- Install/enable Kyverno (or rely on VAP only) and load Cosign keys
- CNI that enforces NetworkPolicy + real apiserver CIDR allowlists
- IdP group mapping and periodic binding reviews
- Choosing ClusterRole vs single-namespace Role for your tenancy model

## Reporting issues

Private disclosure: [SECURITY.md](../SECURITY.md).
