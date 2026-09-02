resource "netgear_vlan" "mgmt" {
  vlan_id = 10
  name    = "mgmt"

  # Uplink carries the VLAN tagged, the two access ports do not.
  tagged_ports   = ["0/24"]
  untagged_ports = ["0/1", "0/2"]
}
