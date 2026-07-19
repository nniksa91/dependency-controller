# Contributing

Thanks for improving **dependency-controller**. This project is a Kubebuilder-based Kubernetes operator that implements Compose-style `depends_on` with typed object references.

## Development setup

Prerequisites:

- Go **1.22+**
- `make`, `docker` (or compatible), `kubectl`
- Access to a cluster for optional smoke tests (Kind is fine)

```sh
git clone https://github.com/nniksa91/dependency-controller.git
cd dependency-controller
make test
make run   # against your current kubeconfig after `make install`
```

## Workflow

1. Open an issue (or claim an existing one) for non-trivial changes.
2. Create a branch from `main`.
3. Keep PRs focused — one concern per PR when possible.
4. Update docs/samples when you change API or behavior.
5. Ensure CI is green.

### Code generation

After editing API types under `api/`:

```sh
make manifests generate
```

Commit the regenerated CRD, RBAC, and deepcopy files.

### Tests

```sh
make test          # unit tests
make lint          # golangci-lint (if installed locally)
```

Prefer table-driven or Ginkgo tests that assert scaling, readiness, and status reasons.

### Commit messages

Use clear, imperative subjects (Conventional Commits welcome):

```
feat: support serviceCompleted for Jobs
fix: avoid panic when replicas is nil
docs: document readyWhen for custom resources
```

Do not add `Co-authored-by: Cursor` (or other Cursor/agent) trailers. This repo ships a `commit-msg` hook that strips those lines. Enable it for your local clone once:

```sh
git config core.hooksPath .githooks
```

## API compatibility

Changes to the `Dependency` CRD are breaking for users. Call them out in the PR title/body and update `CHANGELOG.md`.

## Code of conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Security

Do not file public issues for vulnerabilities. See [SECURITY.md](SECURITY.md).
