# terraform-provider-netgear

## What this is

A Terraform/OpenTofu provider for NETGEAR devices using the Terraform Plugin Framework (protocol 6).
Go module: `github.com/UnstoppableMango/terraform-provider-netgear`.
Provider address: `registry.opentofu.org/UnstoppableMango/netgear`.
The project is nix first: the flake builds the provider with gomod2nix and provides the dev shell with all tooling.

## Commands

- `make build` runs `nix build .#`, which builds via `nix/default.nix` (`buildGoApplication`) and runs Ginkgo in its checkPhase.
- `make test` runs `go tool ginkgo run -r`.
- `make test-acc` runs acceptance tests (`TF_ACC=1 go test ./...`) against OpenTofu using the `TF_ACC_*` variables set by the dev shell.
- `make lint` runs `golangci-lint run ./...` and `nix flake check` (treefmt formatting plus package checks).
- `make fmt` runs `nix fmt` (treefmt: gofmt, nixfmt, mdformat, actionlint, shellcheck).
- `make docs` runs `tfplugindocs generate`; generated files in `docs/` must not be hand-edited and are verified by the CI codegen job.
- `make tidy` regenerates `go.sum` and `nix/gomod2nix.toml`; run it after any dependency change.

## Architecture

- `main.go` serves the provider via `providerserver.Serve`; `version` is injected by goreleaser ldflags.
- `internal/provider/provider.go` holds the `netgearProvider` type; `New(version)` is the only exported constructor.
- Resources and data sources follow the pattern `netgear_<name>_resource.go` / `netgear_<name>_data_source.go` with unexported types, exported `New<Name>Resource` constructors, and interface assertions at the top of each file.
- Tests use Ginkgo/Gomega; every package with tests has a `*_suite_test.go` entrypoint running `RunSpecs`.
- Version is single-sourced from the `VERSION` file: Nix reads it at eval time and release-please bumps it via `extra-files`.

## Releases

- release-please manages versioning and CHANGELOG from conventional commits; never hand-edit `VERSION`, the manifest, or CHANGELOG entries.
- Tag pushes (`v*`) trigger goreleaser, which builds, zips, checksums, and GPG-signs registry artifacts.
- Pending setup: the repo secrets `GPG_PRIVATE_KEY`, `PASSPHRASE`, and `RELEASE_PLEASE_TOKEN` are not configured yet, and the sops-encrypted release key (`secrets/`, `.sops.yaml`) from the terraform-provider-git convention is not present.

## Current state

Skeleton only: empty provider schema, no resources or data sources.
