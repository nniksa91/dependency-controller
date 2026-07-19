# Support

## How to get help

1. Read the [README](README.md), [docs/](docs/), and the [CRD reference](docs/crd-reference.md).
2. Search [existing issues](https://github.com/nniksa91/dependency-controller/issues).
3. Open a [bug report](https://github.com/nniksa91/dependency-controller/issues/new/choose) or feature request.

## What to include

- Kubernetes version and how the controller was installed (`make deploy` vs `.helm/`)
- The `Dependency` CR and related object YAML (redact secrets)
- Controller logs around the failure
- Expected vs actual behavior (include `kubectl get dependency <name> -o yaml` status)

## Security issues

Do not file public issues for vulnerabilities. Follow [SECURITY.md](SECURITY.md).
