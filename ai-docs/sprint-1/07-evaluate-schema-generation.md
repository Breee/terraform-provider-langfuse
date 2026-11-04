# 07 — Evaluate Terraform schema generation

Status: todo (spike, after 06)

## Problem

Resource schemas and Terraform models are hand-written. Generating them from
the spec is the next step towards "generate as much as possible".

## Options

| Tool | Generates | Status |
| --- | --- | --- |
| `tfplugingen-openapi` + `tfplugingen-framework` (HashiCorp) | Plugin Framework schemas, data models, conversions to API types | Tech preview; README: "NOT INTENDED FOR PRODUCTION USE"; latest releases v0.3.0 / v0.4.1, ~2 years old |
| Speakeasy | Full provider incl. CRUD | Commercial, account required; no published OSS/free tier found (2026-09-29) |
| OpenAPI Generator `terraform-provider` | Tag-based stubs | Tested in this repo: compiles, resources are non-functional stubs |

## Scope (spike only)

1. Run `tfplugingen-openapi` on the vendored spec + overlays for one resource
   (`langfuse_llm_connection`), then `tfplugingen-framework generate`.
2. Diff the generated schema against the current one: names, types,
   required/optional/computed, sensitivity, plan modifiers, import format.
3. Write a go/no-go note here.

## Go criteria

- Generated schema is identical to the current one, or differences are fixable
  via generator config without hand-editing output.
- Tool can be pinned and run deterministically in Docker.
- Maintenance risk of an unmaintained preview tool is accepted explicitly.

Otherwise: keep schemas hand-written (Terraform schema is the user-facing
contract; AWS/Azure providers follow the same split).
