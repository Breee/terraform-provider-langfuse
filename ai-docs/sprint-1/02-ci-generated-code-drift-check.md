# 02 — CI: generated-code drift check

Status: completed

CI checks generated clients, mocks, documentation, and examples with
`make check-generated`. Generator and documentation jobs use the Go version
declared by `tools/go.mod`. Dependency updates are handled by Renovate; see
issue [#5](https://github.com/Breee/terraform-provider-langfuse/issues/5) for
human activation and verification. CI runs it with Go from `tools/go.mod` and
Terraform installed.
