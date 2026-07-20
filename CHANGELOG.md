# Go SDK Changelog

## 2.0.0

- The tenant-scoped kernel is the only supported SDK contract and lives at the
  repository root under the semantic-import-versioned module
  `github.com/kave-io/go-sdk/v2`.
- Added typed namespace provisioning, exact consumption, provider route
  activation, OpenAI-compatible transport, usage/invocation/audit/tenant
  reporting, rich token accounting, and recipient-generated service-key
  material.
- Reduced the module to two wire dependencies and one private generated
  Connect contract.
