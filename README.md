# dependency-controller

<p align="center">
  <img src="docs/images/dependency-controller-banner.png" alt="dependency-controller — Compose-style depends_on for Kubernetes" width="100%">
</p>

[![CI](https://github.com/nniksa91/dependency-controller/actions/workflows/ci.yml/badge.svg)](https://github.com/nniksa91/dependency-controller/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nniksa91/dependency-controller)](https://goreportcard.com/report/github.com/nniksa91/dependency-controller)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](go.mod)

Kubernetes operator that brings **Docker Compose–style `depends_on`** to the cluster — with typed object references so you can gate Deployments, StatefulSets, Pods, Jobs, and custom resources.

## Features

- **Typed refs** — `apiVersion` + `kind` + `name` for both sides of the edge
- **Compose conditions** — `serviceStarted`, `serviceHealthy`, `serviceCompleted`
- **Custom resources** — Ready condition or `readyWhen` JSONPath
- **Safe scale gate** — scalable dependents (`Deployment` / `StatefulSet` / `ReplicaSet`) scale to `0` and restore prior replicas
- **Observable status** — `dependencyReady`, `reason`, `message`, and more
- **Dynamic watches** — built-in kinds plus GVKs referenced by live CRs

## Quick example

```yaml
apiVersion: core.example.com/v1
kind: Dependency
metadata:
  name: app-waits-for-db
  namespace: default
spec:
  condition: serviceHealthy
  dependency:
    apiVersion: apps/v1
    kind: StatefulSet
    name: db
  dependent:
    apiVersion: apps/v1
    kind: Deployment
    name: app
```

When `db` has no ready/available replicas, `app` is scaled to `0`. When it recovers, `app` is restored.

More examples: [`config/samples/`](config/samples/) · API details: [`docs/crd-reference.md`](docs/crd-reference.md)

## Documentation

| Doc | Description |
|-----|-------------|
| [Architecture](docs/architecture.md) | Reconcile loop, watches, ready/gate packages |
| [CRD reference](docs/crd-reference.md) | Spec, status, conditions, samples |
| [Security](docs/security.md) | Secure install, RBAC, metrics, supply chain |
| [Helm-style manifests](.helm/README.md) | Flat YAML install without Kustomize |
| [Contributing](CONTRIBUTING.md) | Dev setup and PR expectations |
| [Changelog](CHANGELOG.md) | Notable changes |

## Install

### Prerequisites

- Go 1.22+
- Docker (or compatible)
- `kubectl` and a Kubernetes 1.30+ cluster
- `make`

### Deploy with Kustomize

```sh
make docker-build docker-push IMG=<registry>/dependency-controller:<tag>
make deploy IMG=<registry>/dependency-controller:<tag>
```

### Local development

```sh
make install   # CRDs
make run       # manager against your kubeconfig
```

### Try the sample

```sh
kubectl apply -f .helm/test/pod1-deployment.yaml
kubectl apply -f .helm/test/pod2-deployment.yaml
kubectl apply -f config/samples/core_v1_dependency.yaml
kubectl get dependency -o wide
```

### Uninstall

```sh
kubectl delete -f config/samples/core_v1_dependency.yaml --ignore-not-found
make undeploy
make uninstall
```

## How it works (short)

```
Dependency CR  →  evaluate condition on dependency object
               →  scale / restore scalable dependent
               →  update status
```

| Dependent kind | When dependency is not ready |
|----------------|------------------------------|
| Deployment / StatefulSet / ReplicaSet | Scale to `0` (replicas remembered) |
| Pod / Job / most CRs | Left unchanged; `status.reason=DependentNotScalable` |

Custom dependency kinds need extra RBAC (`get`/`list`/`watch` on that API group; add `update`/`patch` only if the dependent is scaled). Use the placeholder template [`config/rbac/custom_dependency_reader_role.yaml`](config/rbac/custom_dependency_reader_role.yaml) — never grant wildcards. Built-ins are covered by the generated ClusterRole — see [docs/security.md](docs/security.md).

## Development

```sh
make test              # unit tests
make lint              # golangci-lint
make manifests generate
make build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

## Project layout

```
api/v1/                 CRD Go types
cmd/                    Manager entrypoint
internal/controller/    Reconciler + watches
internal/ready/         Compose condition evaluation
internal/gate/          Scale-to-zero / restore
config/                 Kustomize install (CRD, RBAC, manager)
config/samples/         Example Dependency CRs
.helm/                  Flat YAML + demo Deployments
docs/                   Architecture and API docs
```

## Compatibility

| Component | Version |
|-----------|---------|
| Go | 1.22+ |
| Kubernetes | 1.30+ (envtest / CI target) |
| controller-runtime | v0.18.x |

## License

[MIT](LICENSE) © Nikola Niksa

## Security

Private vulnerability reporting: [SECURITY.md](SECURITY.md). Operator hardening: [docs/security.md](docs/security.md).
