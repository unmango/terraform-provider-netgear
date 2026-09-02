data "netgear_interfaces" "all" {}

# The ports with a live link.
output "connected_ports" {
  value = [
    for port in data.netgear_interfaces.all.interfaces : port.port
    if port.link_status == "up"
  ]
}
