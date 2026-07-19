# `.helm/` — flat YAML install (not a Helm chart)

## Install

```sh
kubectl apply -f .helm/crd.yaml
kubectl apply -f .helm/ServiceAccount.yaml
kubectl apply -f .helm/Role.yaml
kubectl apply -f .helm/role_binding.yaml
kubectl apply -f .helm/deployment.yaml
```

`Role.yaml` includes get/list/watch/update/patch for Deployments, StatefulSets, ReplicaSets; get/list/watch for Pods and Jobs; and Dependency status permissions.

For **custom resource** dependencies, add rules for those API groups to `Role.yaml` (or the Kustomize ClusterRole).

## Demo

```sh
kubectl apply -f .helm/test/pod1-deployment.yaml
kubectl apply -f .helm/test/pod2-deployment.yaml
kubectl apply -f .helm/dependency-deployment.yaml
kubectl get dependency my-dependency -o yaml
```

See [docs/crd-reference.md](../docs/crd-reference.md) for StatefulSet / Job / CR examples.
