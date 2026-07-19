# Security policy

## Supported versions

Security fixes are applied to the latest release on `main` and the most recent tagged release when tags exist.

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

## Scope notes

This operator updates Kubernetes objects (primarily `spec.replicas` on scalable workloads). Misconfigured RBAC that grants the controller access to sensitive custom resources increases blast radius — treat ClusterRole rules as part of your security review.
