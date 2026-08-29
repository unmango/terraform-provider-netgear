resource "netgear_interface" "workstation" {
  port        = "0/1"
  description = "workstation"
  enabled     = true
  speed       = "auto"
  pvid        = netgear_vlan.mgmt.vlan_id
}
