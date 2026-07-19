# `.helm/` — flat YAML install (not a Helm chart)

## Install

```sh
kubectl apply -f .helm/crd.yaml
kubectl apply -f .helm/ServiceAccount.yaml
kubectl apply -f .helm/Role.yaml
kubectl apply -f .helm/role_binding.yaml
kubectl apply -f .helm/deployment.yaml
```

`Role.yaml` is a **ClusterRole** (despite the filename) with least-privilege rules aligned to `config/rbac/role.yaml`:

- apps (`deployments` / `statefulsets` / `replicasets`): `get` / `list` / `watch` / `update` / `patch`
- pods & jobs: `get` / `list` / `watch` only
- `dependencies` + `/status`: `get` / `list` / `watch` / `update` / `patch` (no create/delete)

For custom-resource dependencies, edit placeholders in [`config/rbac/custom_dependency_reader_role.yaml`](../config/rbac/custom_dependency_reader_role.yaml) and merge those rules into `Role.yaml` (or bind that ClusterRole to `dependency-controller-sa`). Readiness-only CRs need `get`/`list`/`watch`; scalable dependents also need `update`/`patch`. Never use wildcards.

`deployment.yaml` runs the manager as non-root (`runAsUser: 65532`) with dropped capabilities, read-only rootfs, and `RuntimeDefault` seccomp. The ServiceAccount token is automounted only for this controller Pod. Metrics stay disabled unless you pass `--metrics-bind-address` (prefer HTTPS + `--metrics-secure=true`). Pin the image by digest in production.

These manifests do **not** create a Namespace. For PSA **restricted**, label the target namespace (Kustomize `make deploy` already labels `dependency-system`):

```sh
kubectl label ns default \
  pod-security.kubernetes.io/enforce=restricted \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted \
  --overwrite
```

Optional packs (Kustomize, off by default): NetworkPolicy (`config/network-policy/`), admission / cosign examples (`config/policy/`), single-namespace controller Role (`config/rbac/namespaced/`). Full notes: [docs/security.md](../docs/security.md).

## Demo

Minimal Deploy → Deploy sample:

```sh
kubectl apply -f .helm/test/pod1-deployment.yaml
kubectl apply -f .helm/test/pod2-deployment.yaml
kubectl apply -f .helm/dependency-deployment.yaml
kubectl get dependency my-dependency -o yaml
```

Longer demos (scale gate thesis):

- [`config/samples/scenario-postgres-app/`](../config/samples/scenario-postgres-app/) — Postgres + app; `./hack/test-postgres-app-dependency.sh`
- [`config/samples/scenario-app-waits-for-db/`](../config/samples/scenario-app-waits-for-db/) — synthetic slow DB; `./hack/test-slow-db.sh`

See [docs/crd-reference.md](../docs/crd-reference.md) for StatefulSet / Job / CR examples.
