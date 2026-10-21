<!-- Keep PRs small and focused — one rule, adapter method, fix, or renderer at a time. -->

## Summary

<!-- What does this change do, and why? -->

## Related issues

<!-- e.g. Closes #123 -->

## Type of change

- [ ] Bug fix
- [ ] New feature (rule, adapter, command, renderer, …)
- [ ] Refactor / internal change
- [ ] Docs only

## Checklist

- [ ] `make fmt vet test` passes locally (gofmt clean, vet clean, tests green)
- [ ] Added or updated tests (rules include a positive and a negative fixture)
- [ ] New `.go` files carry the Apache-2.0 `Copyright 2026 S3I Groups LLC` header
- [ ] No secrets, credentials, real endpoints, or customer data in code or fixtures
- [ ] Core stays vendor-neutral (no platform-specific types leak past `PlatformAdapter`)
- [ ] Finding IDs are new (never renumbered an existing one)

## Notes for reviewers

<!-- Anything reviewers should focus on, trade-offs, or follow-ups. -->
