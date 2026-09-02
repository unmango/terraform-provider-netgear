# terraform-provider-netgear

## What this is

A Terraform/OpenTofu provider for NETGEAR devices using the Terraform Plugin Framework (protocol 6).
Go module: `github.com/unmango/terraform-provider-netgear`.
Provider address: `registry.opentofu.org/unmango/netgear`.
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
- Artifacts are signed by a dedicated release key, `terraform-provider-netgear release signing`, fingerprint `28BBEF32809BE4D75E5AAA2B152839F1CCF000D0`. The private key and its passphrase live in `secrets/gpg-release-key.enc.yaml`, encrypted to the personal key whose fingerprint `.sops.yaml` lists; read them with `sops --decrypt secrets/gpg-release-key.enc.yaml`. The same values are set as the repo secrets `GPG_PRIVATE_KEY` and `PASSPHRASE`.
- Pending setup: `RELEASE_PLEASE_TOKEN` is not configured. The action needs a PAT rather than `GITHUB_TOKEN`, because a tag pushed with `GITHUB_TOKEN` does not trigger the goreleaser workflow. The public half of the signing key also has to be registered with the provider registry before a release verifies.

## Current state

`internal/fastpath` speaks the FASTPATH CLI over telnet: login flow detection, telnet option refusal, prompt matching, CLI error detection, NVRAM save, and parsers for `show running-config` and `show port`.
`netgear_vlan`, `netgear_interface`, and `netgear_lag` are implemented against it.
Each has a matching data source, plus a plural `netgear_vlans`, `netgear_interfaces`, and `netgear_lags` that list what the switch has.
The plural interface data source enumerates ports from `show port all` rather than the running config, because a port left at its defaults is not printed in the config.

Ports are named two ways by the firmware, `g7` and `0/7`. Everything read off the switch is normalized to the slot form by `fastpath.NormalizePort`; see `docs/management-interfaces.md` for what has been verified on hardware.

Resource CRUD is covered by specs that assert the exact command list sent to the switch, using the `fakeClient` in `internal/provider/fake_client_test.go` and the helpers in `crud_helpers_test.go`.
