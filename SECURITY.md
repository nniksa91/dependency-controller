# Security policy

## Supported versions

Security fixes are applied to the latest commit on `main` and the most recent tagged release when tags exist. Older tags are best-effort only.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Instead, report privately via one of:

- GitHub Security Advisories for [nniksa91/dependency-controller](https://github.com/nniksa91/dependency-controller/security/advisories/new)
- Email the maintainer listed in [MAINTAINERS.md](MAINTAINERS.md)

Include:

- Description of the issue and impact
- Steps to reproduce or a proof of concept
- Affected versions / commit SHAs if known

You should receive an acknowledgement within a few days. We will coordinate a fix and disclosure timeline.

## Operator security posture

| Area | Expectation |
|------|-------------|
| RBAC | Least privilege ClusterRole for built-in kinds; no wildcards; human editor/viewer/creator roles separate from controller SA |
| Container | Distroless non-root image; read-only rootfs; drop ALL caps; RuntimeDefault seccomp; PSA restricted labels on Kustomize install NS |
| Metrics | Off by default (helm); HTTPS + authn/authz when enabled via Kustomize |
| Network | Optional NetworkPolicy component (`config/network-policy/`) |
| Admission | Optional VAP / Kyverno + cosign EXAMPLE (`config/policy/`) — off by default |
| Secrets | Operator does not consume app Secrets; redact them in reports |
| License | MIT — see [LICENSE](LICENSE) |

Install and hardening details: [docs/security.md](docs/security.md).

## Scope notes

This operator updates Kubernetes objects (primarily `spec.replicas` on scalable workloads). Misconfigured RBAC that grants the controller access to sensitive custom resources increases blast radius — treat ClusterRole rules as part of your security review.
