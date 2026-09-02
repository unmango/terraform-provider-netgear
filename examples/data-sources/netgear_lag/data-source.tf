data "netgear_lag" "uplink" {
  lag_id = 1
}

# Put the group in a VLAN by referencing the interface id the switch gives it.
resource "netgear_vlan" "mgmt" {
  vlan_id      = 10
  tagged_ports = [data.netgear_lag.uplink.interface_id]
}
