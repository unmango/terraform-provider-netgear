data "netgear_vlans" "all" {}

output "vlan_ids" {
  value = [for vlan in data.netgear_vlans.all.vlans : vlan.vlan_id]
}
