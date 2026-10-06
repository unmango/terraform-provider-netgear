# Changelog

## [0.1.1](https://github.com/unmango/terraform-provider-netgear/compare/v0.1.0...v0.1.1) (2026-10-06)


### Tests

* drive the switch through acceptance tests ([#6](https://github.com/unmango/terraform-provider-netgear/issues/6)) ([df6f772](https://github.com/unmango/terraform-provider-netgear/commit/df6f7723cb9d2ceb2d7d9f85f6e5c55f40a80556))


### Continuous Integration

* tag releases with the release app so goreleaser runs ([#30](https://github.com/unmango/terraform-provider-netgear/issues/30)) ([1bbd52a](https://github.com/unmango/terraform-provider-netgear/commit/1bbd52a76ff31baccd42cd60d027fc53a0192386))

## 0.1.0 (2026-09-28)


### Features

* add netgear_vlan resource scaffold with schema and validation ([de717db](https://github.com/unmango/terraform-provider-netgear/commit/de717dbb4806a09b5d842c7e0588d430f063c2c3))
* add shared client interface and provider helper utilities ([de717db](https://github.com/unmango/terraform-provider-netgear/commit/de717dbb4806a09b5d842c7e0588d430f063c2c3))
* implement the provider against the FASTPATH CLI ([#1](https://github.com/unmango/terraform-provider-netgear/issues/1)) ([de717db](https://github.com/unmango/terraform-provider-netgear/commit/de717dbb4806a09b5d842c7e0588d430f063c2c3))
* **provider:** add netgear LAG resource scaffold with schema and tests ([de717db](https://github.com/unmango/terraform-provider-netgear/commit/de717dbb4806a09b5d842c7e0588d430f063c2c3))
* **provider:** add netgear_interface resource scaffold with schema and tests ([de717db](https://github.com/unmango/terraform-provider-netgear/commit/de717dbb4806a09b5d842c7e0588d430f063c2c3))
* **provider:** implement provider schema, configuration, and resource registration ([de717db](https://github.com/unmango/terraform-provider-netgear/commit/de717dbb4806a09b5d842c7e0588d430f063c2c3))


### Bug Fixes

* bump grpc past the advisories ([#5](https://github.com/unmango/terraform-provider-netgear/issues/5)) ([0b6d5c1](https://github.com/unmango/terraform-provider-netgear/commit/0b6d5c1d251d1ea3d00174953862319b728bc8f3))
* **deps:** update module github.com/onsi/ginkgo/v2 to v2.33.0 ([#18](https://github.com/unmango/terraform-provider-netgear/issues/18)) ([ec78a9f](https://github.com/unmango/terraform-provider-netgear/commit/ec78a9fcb1a6d2c25167ba54a92009a529b0f546))
* **renovate:** reference shared presets by name ([#24](https://github.com/unmango/terraform-provider-netgear/issues/24)) ([c16d539](https://github.com/unmango/terraform-provider-netgear/commit/c16d5395ab68ac4047ee74212cdfcb568d5f1ed1)), closes [#23](https://github.com/unmango/terraform-provider-netgear/issues/23)


### Documentation

* add Hercules CI badge ([#22](https://github.com/unmango/terraform-provider-netgear/issues/22)) ([dbecc79](https://github.com/unmango/terraform-provider-netgear/commit/dbecc797c3ccaeb2915e537399545194698ba25f))


### Continuous Integration

* adopt unmango/actions ([#21](https://github.com/unmango/terraform-provider-netgear/issues/21)) ([136993f](https://github.com/unmango/terraform-provider-netgear/commit/136993f71a385b3ab46c2a0fbfeb827dc8753e1c))
