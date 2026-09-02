data "netgear_interface" "uplink" {
  port = "0/24"
}

output "uplink_link_status" {
  value = data.netgear_interface.uplink.link_status
}
