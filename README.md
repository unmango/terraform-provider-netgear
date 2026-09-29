# terraform-provider-netgear

[![Hercules CI](https://hercules-ci.com/api/v1/site/github/account/unmango/project/terraform-provider-netgear/badge)](https://hercules-ci.com/github/unmango/terraform-provider-netgear)

A Terraform/OpenTofu provider for NETGEAR devices, built with the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework).

## Status

Working against a GS724Tv4 over the FASTPATH CLI.

Resources: `netgear_vlan`, `netgear_interface`, `netgear_lag`.
Data sources: `netgear_vlan`, `netgear_vlans`, `netgear_interface`, `netgear_interfaces`, `netgear_lag`, `netgear_lags`.

See [`docs/management-interfaces.md`](docs/management-interfaces.md) for what has been verified on hardware.

## Usage

```terraform
terraform {
  required_providers {
    netgear = {
      source = "unmango/netgear"
    }
  }
}

provider "netgear" {}
```

## Development

This repo is nix first.
`direnv allow` loads the dev shell automatically, or use `nix develop`.

Common tasks are wrapped in the Makefile:

```sh
make build     # nix build .#
make test      # go tool ginkgo run -r
make test-acc  # TF_ACC=1 go test ./...
make lint      # golangci-lint + nix flake check
make fmt       # nix fmt (treefmt)
make docs      # tfplugindocs generate
make tidy      # regenerate go.sum and nix/gomod2nix.toml
```

Run `make tidy` after changing dependencies so `nix/gomod2nix.toml` stays in sync with `go.mod`.

Acceptance tests run against OpenTofu and a real switch.
The dev shell sets `TF_ACC_TERRAFORM_PATH` and related variables for you; set `NETGEAR_HOST` and `NETGEAR_PASSWORD` to point them at hardware.
Without those they skip, so `make test-acc` is safe to run anywhere.
The tests change the VLAN, ports, and LAG named by `NETGEAR_ACC_VLAN_ID`, `NETGEAR_ACC_PORT`, `NETGEAR_ACC_PORT_2`, and `NETGEAR_ACC_LAG_ID`, whose defaults (4090, 0/26, 0/25, 26) are unused only on the reference GS724Tv4, so set them to identifiers your switch does not use.

## License

[MIT](LICENSE)
