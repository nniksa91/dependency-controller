# Documentation

| Document | Description |
|----------|-------------|
| [architecture.md](architecture.md) | Reconcile loop, watches, ready/gate packages, RBAC |
| [crd-reference.md](crd-reference.md) | `Dependency` API, conditions, samples |
| [security.md](security.md) | Secure install, PSA, RBAC, optional NetworkPolicy / admission / cosign |

## Demo scenarios

| Scenario | What it shows |
|----------|----------------|
| [`scenario-postgres-app/`](../config/samples/scenario-postgres-app/) | **Real stack** — Postgres + app (no app probes); Dependency keeps app at 0 until DB Available → avoids CrashLoop and static probe delays |
| [`scenario-app-waits-for-db/`](../config/samples/scenario-app-waits-for-db/) | Synthetic slow DB (nginx + sleep) for a short replica timeline |

Optional cluster policy artifacts (off by default): [`config/policy/`](../config/policy/) · NetworkPolicy: [`config/network-policy/`](../config/network-policy/) · Single-NS RBAC: [`config/rbac/namespaced/`](../config/rbac/namespaced/)

Project overview and quick start: [../README.md](../README.md)

Contributing: [../CONTRIBUTING.md](../CONTRIBUTING.md) · Changelog: [../CHANGELOG.md](../CHANGELOG.md) · Vulnerability reporting: [../SECURITY.md](../SECURITY.md)
