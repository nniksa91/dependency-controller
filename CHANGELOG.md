# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Multi-kind typed `ObjectRef` for `spec.dependency` and `spec.dependent` (`apiVersion` / `kind` / `name`) instead of Deployment-only name strings
- Compose-style readiness conditions: `serviceStarted`, `serviceHealthy`, `serviceCompleted`
- `readyWhen` JSONPath evaluation for custom-resource dependencies
- Scale gating for `Deployment`, `StatefulSet`, and `ReplicaSet` dependents with prior-replica preservation
- Dynamic informer watches for built-in kinds and GVKs referenced by live `Dependency` CRs
- Enriched status fields (`dependencyReady`, `dependentScaledDown`, `reason`, `message`, `condition`, `observedGeneration`)
- Unit tests covering Deployment / StatefulSet / custom-resource paths and readiness evaluators
- GitHub Actions CI and release workflows, issue/PR templates, Dependabot, and community docs (`CONTRIBUTING`, `SECURITY`, `CODE_OF_CONDUCT`, `SUPPORT`)
- Demo scenarios with cluster test scripts:
  - Postgres + app scale gate with a probe-free app dependent (`config/samples/scenario-postgres-app/`, `hack/test-postgres-app-dependency.sh`)
  - Synthetic slow DB for a short replica timeline (`config/samples/scenario-app-waits-for-db/`, `hack/test-slow-db.sh`)
- Optional zero-trust install hardening: NetworkPolicy component (`config/network-policy/`), ValidatingAdmissionPolicy / Kyverno example packs (`config/policy/`), PSA `restricted` labels on the Kustomize install namespace, `dependency-creator-role`, and namespaced controller RBAC (`config/rbac/namespaced/`)
- Local `commit-msg` hook under `.githooks/` to strip unwanted `Co-authored-by` trailers from commits

### Changed

- **Breaking:** `spec.dependency` / `spec.dependent` are typed `ObjectRef` objects, not plain Deployment name strings
- Product docs emphasize the scale-gate thesis: keep dependents at `0` until the dependency is ready so apps need not rely on static probe delays or CrashLoop while waiting
- Documentation professional pass: README support matrix and API-group note, architecture / CRD / security guides, sample scenario READMEs, and Helm-style install notes
- Dependabot: stabilize updates and pin ignores for Kubernetes module bumps that break CI; unblock main lint
- Align MIT license headers on generated/API touchpoints

### Fixed

- Nil-safe handling of `spec.replicas` on scalable dependents
- CR-primary reconcile so applying a `Dependency` takes effect without a manual Deployment poke

### Removed

- Tracked `bin/` tool binaries (`controller-gen` and related symlinks) from version control; `/bin/` remains gitignored for local builds

### Security

- Harden default controller RBAC to least privilege for built-in kinds (no wildcards); document multi-tenant blast radius and optional single-namespace install
- Distroless non-root manager defaults, read-only rootfs, dropped capabilities, RuntimeDefault seccomp; metrics off by default unless explicitly enabled
- Optional NetworkPolicy and admission/policy packs remain off by default and must be reviewed before enablement

## [0.1.0] - 2024-01-01

### Added

- Initial Kubebuilder operator scaffolding and Deployment-only dependency prototype
