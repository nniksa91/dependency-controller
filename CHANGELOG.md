# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Typed `ObjectRef` dependency/dependent fields (`apiVersion` / `kind` / `name`)
- Compose-style conditions: `serviceStarted`, `serviceHealthy`, `serviceCompleted`
- `readyWhen` JSONPath support for custom resources
- Scale gating for `Deployment`, `StatefulSet`, and `ReplicaSet` with replica preservation
- Dynamic watches for GVKs referenced by Dependency CRs
- Enriched status (`reason`, `message`, `condition`, `observedGeneration`)
- Unit tests for Deploy/STS/CR paths and readiness evaluators
- GitHub Actions CI, issue/PR templates, Dependabot, and community docs

### Changed

- **Breaking:** `spec.dependency` / `spec.dependent` are no longer plain Deployment name strings

### Fixed

- Nil-safe handling of `spec.replicas` on scalable dependents
- CR-primary reconcile so applying a Dependency CR takes effect without a manual Deployment poke

## [0.1.0] - 2024-01-01

### Added

- Initial Kubebuilder operator scaffolding and Deployment-only dependency prototype
