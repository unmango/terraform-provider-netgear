data "netgear_lags" "all" {}

output "lag_members" {
  value = { for lag in data.netgear_lags.all.lags : lag.lag_id => lag.members }
}
