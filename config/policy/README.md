# Optional admission / image policy pack

**Disabled by default.** Nothing here is applied by `make deploy`. Requires cluster-side tooling (Kyverno and/or Kubernetes ValidatingAdmissionPolicy). Default installs must keep working without Kyverno.

| Path | What it does | Prerequisite |
|------|----------------|--------------|
| [`vap/`](vap/) | Restrict who may create/update `Dependency` CRs (built-in VAP) | Kubernetes 1.30+ |
| [`kyverno/`](kyverno/) | Same creator restriction + **EXAMPLE** cosign `verifyImages` | [Kyverno](https://kyverno.io/) installed |

## Quick enable

```sh
# Built-in ValidatingAdmissionPolicy (edit allowed groups first)
kubectl apply -k config/policy/vap

# Kyverno examples (edit groups + cosign public key first)
kubectl apply -k config/policy/kyverno
```

Or uncomment the commented resource entries in [`config/default/kustomization.yaml`](../default/kustomization.yaml) after reviewing placeholders.

## Placeholders you must edit

- **Allowed groups** — replace `dependency-creators` (and any examples) with your IdP / RBAC groups.
- **Controller SA exclusion** — default match conditions skip `system:serviceaccount:dependency-system:*` so the manager can still patch Dependency objects. Change the namespace/SA prefix if you rename the install.
- **Cosign** — `kyverno/verify-images-example.yaml` is an **EXAMPLE** only. Insert your real public key (or keyless OIDC issuer/subject) before Enforce mode.
- **Mode** — VAP binding and Kyverno policies ship in `Audit`; flip to Deny / Enforce only after dry-runs.

## ObjectRef / cross-namespace

`ObjectRef` has no `namespace` field; the controller resolves refs in the **Dependency CR's namespace**. Admission cannot "deny cross-namespace refs" as a field check — isolation is:

1. Who can create Dependency CRs in which namespaces (this pack + `dependency-creator-role`).
2. Whether the controller SA is cluster-scoped or single-namespace (`config/rbac/namespaced/`).

See [docs/security.md](../../docs/security.md).
