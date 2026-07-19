# `.helm/` — flat YAML install (not a Helm chart)

## Install

```sh
kubectl apply -f .helm/crd.yaml
kubectl apply -f .helm/ServiceAccount.yaml
kubectl apply -f .helm/Role.yaml
kubectl apply -f .helm/role_binding.yaml
kubectl apply -f .helm/deployment.yaml
```

`Role.yaml` is a ClusterRole with get/list/watch/update/patch for Deployments, StatefulSets, ReplicaSets; get/list/watch for Pods and Jobs; and Dependency status permissions.

For **custom resource** dependencies, add rules for those API groups to `Role.yaml` (or the Kustomize ClusterRole) — readiness-only CRs need `get`/`list`/`watch`; scalable dependents also need `update`/`patch`.

`deployment.yaml` runs the manager as non-root with dropped capabilities and `RuntimeDefault` seccomp. Metrics stay disabled unless you pass `--metrics-bind-address` (prefer HTTPS + `--metrics-secure=true`). See [docs/security.md](../docs/security.md).

## Demo

```sh
kubectl apply -f .helm/test/pod1-deployment.yaml
kubectl apply -f .helm/test/pod2-deployment.yaml
kubectl apply -f .helm/dependency-deployment.yaml
kubectl get dependency my-dependency -o yaml
```

See [docs/crd-reference.md](../docs/crd-reference.md) for StatefulSet / Job / CR examples.
