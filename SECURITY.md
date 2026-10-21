# Security Policy

## Reporting a vulnerability

Please report security issues **privately** — do not open a public issue or
pull request for a vulnerability.

Use GitHub's **private vulnerability reporting**: go to the repository's
**Security** tab → **Report a vulnerability**. This opens a private advisory
visible only to the maintainers.

When reporting, please include:

- a description of the issue and its impact,
- steps to reproduce (a minimal example helps),
- affected version or commit, and
- any suggested fix, if you have one.

## What to expect

- We aim to acknowledge a report within a few business days.
- We will keep you updated as we investigate and work on a fix.
- Once a fix is available, we will coordinate disclosure and credit you if you'd
  like to be named.

## Scope

Flinkit is a read-only diagnostic CLI: it reads from Flink REST APIs and the
Kubernetes API and never deploys or mutates workloads by default. Reports that
are especially relevant include credential handling, the Kubernetes transports
(apiserver proxy, service DNS), and any path that could leak configuration or
connection details.
