# Single-namespace controller RBAC

Default `make deploy` installs a **ClusterRole** so the manager can `list`/`watch` across namespaces. That is the wider blast-radius path: anyone who can create a `Dependency` CR in *any* namespace the controller watches can cause scale mutations on Deployments/StatefulSets/ReplicaSets the SA can touch **cluster-wide**.

If you run the operator for **one trusted namespace only**, use this Role + RoleBinding instead of the cluster-scoped binding.

## Enable (manual)

1. Deploy CRDs and the manager into the target namespace (adjust `config/default` `namespace:` / helm `namespace:` as needed).
2. **Do not** apply `config/rbac/role_binding.yaml` (ClusterRoleBinding), or delete it after a trial install:
   ```sh
   kubectl delete clusterrolebinding dependency-manager-rolebinding --ignore-not-found
   ```
3. Apply this package in the same namespace as the controller SA:
   ```sh
   kubectl apply -k config/rbac/namespaced -n <controller-namespace>
   ```
   If you use `namePrefix` / renamed SA, edit `role_binding.yaml` subjects to match.
4. Keep the ClusterRole `manager-role` unused, or delete it if nothing else references it:
   ```sh
   kubectl delete clusterrole dependency-manager-role --ignore-not-found
   ```

Leader-election and metrics-auth bindings are unchanged (namespaced Role + TokenReview ClusterRole).

## Sync note

Rules here mirror `config/rbac/role.yaml`. After `make manifests` changes the ClusterRole, update this Role to match.
