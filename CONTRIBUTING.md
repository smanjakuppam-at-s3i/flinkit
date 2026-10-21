# Contributing to Flinkit

Thanks for your interest in Flinkit — a vendor-neutral CLI for diagnosing
Apache Flink jobs. Contributions are welcome. This guide covers how to build,
test, and submit changes.

## Getting started

You need **Go 1.23+**. Then:

```sh
git clone https://github.com/smanjakuppam-at-s3i/flinkit.git
cd flinkit
make build     # builds ./flinkit
make test      # runs all tests
```

Common targets (`make help` lists them): `build`, `install`, `test`, `vet`,
`fmt`, `tidy`, `clean`.

## Before you open a pull request

Make sure the tree is clean and green:

```sh
make fmt       # gofmt -w
make vet       # go vet
make test      # go test ./...
```

CI runs `gofmt` (must report no diffs), `go vet`, `go build`, and `go test`.
A PR won't be merged until those pass.

## Conventions

- **Keep PRs small and focused** — one rule, one adapter method, one fix, or one
  renderer change at a time. Small PRs get reviewed and merged faster.
- **Add tests with behavior.** Rules ship with at least one positive and one
  negative fixture under `testdata/`. Adapters are tested against recorded API
  responses (golden files), not live clusters; live tests sit behind a build
  tag and are skipped in CI.
- **License header on new source files.** Every `.go` file carries the standard
  Apache-2.0 header:

  ```go
  // Copyright 2026 S3I Groups LLC
  //
  // Licensed under the Apache License, Version 2.0 (the "License");
  // ...
  ```

- **Keep the core vendor-neutral.** Platform-specific types never leak past the
  `PlatformAdapter` interface. Every platform (OSS REST, Kubernetes, …) is an
  adapter behind that seam.
- **Stable finding IDs.** Once released, IDs like `DOC-CKPT-001` or
  `BTL-SKEW-001` are never renumbered.
- **No secrets, ever.** No credentials, kubeconfigs, real endpoints, or customer
  data in code or fixtures. Use synthetic workloads only; connection details
  live in `~/.flinkit/config.yaml` and reference env vars as `${VAR}`.

## Submitting

1. Fork the repo and create a branch off `main`.
2. Make your change with tests; run `make fmt vet test`.
3. Open a pull request and fill out the template. Link any related issue.
4. A maintainer reviews; please address feedback by pushing follow-up commits.

## Reporting bugs and security issues

- **Bugs / feature ideas:** open a GitHub issue with steps to reproduce and the
  Flink version / platform involved.
- **Security vulnerabilities:** do **not** open a public issue — see
  [SECURITY.md](SECURITY.md) for private reporting.

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE), the same license that covers this project.
