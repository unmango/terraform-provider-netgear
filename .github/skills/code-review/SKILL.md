---
name: code-review
description: Review changes to terraform-provider-netgear against verified GS724Tv4 FASTPATH behaviour and the repo's Nix and release conventions. Use when reviewing a pull request in this repository.
---

This provider drives the undocumented Broadcom FASTPATH CLI over telnet on port 60000.
Almost every wrong review of it comes from assuming the CLI behaves the way a Cisco-style CLI would, or from reading an absence in the running config as a deletion.

## Switch behaviour that decides whether a finding is real

`docs/management-interfaces.md` records what has been run against the reference hardware, a GS724Tv4 on firmware 6.3.1.19.
Treat it as the source of truth and cite it.
A claim about the switch that contradicts it is wrong; a claim it does not cover is unverified.

- Ports have two spellings, `g7` and `0/7`, and so do LAGs, `lag 1` and `3/1`.
  Everything read off the switch is normalized to the slot form by `fastpath.NormalizePort` in `internal/fastpath/port.go`.
  A comparison against a raw short-form id is a bug.
- `show running-config` prints only what differs from the default.
  A port or LAG left entirely at its defaults is absent from it, and VLAN 1 never appears at all.
  Absent means default, not gone.
  A `Read` that removes the resource from state because the stanza is missing makes Terraform recreate it forever; `Read` in `internal/provider/netgear_interface_resource.go` shows the pattern, consulting `show port` before removing anything.
- Ports, LAG interfaces, and VLAN 1 are not created or destroyed.
  The switch defines lag 1 through lag 26 whether or not they are used, and `no interface lag 1` is rejected exactly as deleting a port would be.
  Resources adopt what already exists, and `Delete` returns the settings to their defaults.
- A command that does not appear in `docs/management-interfaces.md` has not been tried on this firmware.
  Say it needs hardware verification rather than asserting it works.
  `staticcapability` and `hashing-mode` are the standing example: both read as obvious, neither exists here.
- Only `show port` carries an admin mode column.
  `show interfaces status` reports the link state alone.

## Codebase conventions

- CRUD is covered by specs that assert the exact list of commands sent to the switch, using the `fakeClient` in `internal/provider/fake_client_test.go` and the helpers in `internal/provider/crud_helpers_test.go`.
  A change that sends a new command needs a spec in the matching `netgear_*_crud_test.go` or `*_read_test.go`.
- Ginkgo and Gomega throughout, one `*_suite_test.go` per package.
- Resources and data sources use unexported types with `New<Name>Resource` and `New<Name>DataSource` constructors, and interface assertions at the top of the file.
- Shared model helpers live in `internal/provider/models.go`: `stringSet`, `int64Set`, `sortedPorts`, `stringOrNull`, `int64OrNull`, `quote`, `itoa`.
  Prefer them to a new local helper.

## Tooling and release

- `go.mod` or `go.sum` changed without `nix/gomod2nix.toml` means `make tidy` was not run, and the Nix build will fail.
- `docs/` is `tfplugindocs` output, regenerated with `make docs`.
  It is never hand-edited, and CI verifies it.
- `VERSION`, `.release-please-manifest.json`, and `CHANGELOG.md` belong to release-please.

## Do not flag

- Telnet without transport security.
  It is the only interface with enough coverage to back real resources on this hardware.
  The provider schema and the docs already say to reach the switch over a management VLAN only.
- `secrets/*.enc.yaml`.
  Encrypted with sops to the recipient in `.sops.yaml`, deliberately committed, not leaked key material.
- The `go` directive in `go.mod`.
  A patch version such as `go 1.26.5` has been legal since Go 1.21.
- Anything generated under `docs/`.
