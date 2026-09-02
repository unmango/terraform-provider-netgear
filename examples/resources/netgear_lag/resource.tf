resource "netgear_lag" "uplink" {
  lag_id  = 1
  name    = "uplink"
  mode    = "lacp"
  members = ["0/23", "0/24"]
}
