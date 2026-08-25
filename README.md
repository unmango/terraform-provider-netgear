# terraform-provider-netgear

A Terraform/OpenTofu provider for NETGEAR devices, built with the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework).

## Status

Early skeleton.
The provider builds and serves, but no resources or data sources are implemented yet.

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

Acceptance tests run against OpenTofu.
The dev shell sets `TF_ACC_TERRAFORM_PATH` and related variables for you.

## License

[MIT](LICENSE)
