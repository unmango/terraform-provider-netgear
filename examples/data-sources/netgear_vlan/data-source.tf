data "netgear_vlan" "mgmt" {
  vlan_id = 10
}

output "mgmt_uplinks" {
  value = data.netgear_vlan.mgmt.tagged_ports
}
