package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Acceptance tests drive OpenTofu against a real switch. There is no fake:
// the point of these is the firmware, and the FASTPATH CLI is undocumented
// enough that a pass against a simulator would only confirm what this repo
// already believes.
//
// They run when TF_ACC and the connection variables are set, and skip
// otherwise, which is why CI stays green without hardware. The dev shell
// supplies TF_ACC_TERRAFORM_PATH; `make test-acc` supplies TF_ACC.
//
// Nothing here asserts a CheckDestroy. Ports, LAG interfaces, and VLAN 1 are
// not created or destroyed on this hardware, so destroying a resource returns
// its settings to their defaults rather than removing anything.

// testAccPreCheck skips a test unless the switch connection is configured.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	for _, name := range []string{"NETGEAR_HOST", "NETGEAR_PASSWORD"} {
		if os.Getenv(name) == "" {
			t.Skipf("%s is not set, skipping the acceptance test", name)
		}
	}
}

// accEnv reads a test parameter, falling back to a value the reference
// GS724Tv4 leaves unused.
func accEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

// The defaults target what the reference switch does not use for anything
// real: a VLAN id well clear of the configured 20, 69, and 100, and the two
// SFP ports.
func accVlanID() string { return accEnv("NETGEAR_ACC_VLAN_ID", "4090") }
func accPort() string   { return accEnv("NETGEAR_ACC_PORT", "0/26") }
func accPort2() string  { return accEnv("NETGEAR_ACC_PORT_2", "0/25") }
func accLagID() string  { return accEnv("NETGEAR_ACC_LAG_ID", "26") }

func TestAccNetgearVlan_basic(t *testing.T) {
	testAccPreCheck(t)

	id, port := accVlanID(), accPort()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "netgear_vlan" "test" {
  vlan_id        = %[1]s
  name           = "tf acc"
  untagged_ports = [%[2]q]
}`, id, port),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netgear_vlan.test", "id", id),
					resource.TestCheckResourceAttr("netgear_vlan.test", "name", "tf acc"),
					resource.TestCheckResourceAttr("netgear_vlan.test", "untagged_ports.#", "1"),
					resource.TestCheckTypeSetElemAttr("netgear_vlan.test", "untagged_ports.*", port),
				),
			},
			// Moving the port from untagged to tagged exercises the membership
			// reconciliation, which sends `vlan tagging` rather than rebuilding
			// the whole VLAN.
			{
				Config: fmt.Sprintf(`resource "netgear_vlan" "test" {
  vlan_id      = %[1]s
  name         = "tf acc renamed"
  tagged_ports = [%[2]q]
}`, id, port),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netgear_vlan.test", "name", "tf acc renamed"),
					resource.TestCheckTypeSetElemAttr("netgear_vlan.test", "tagged_ports.*", port),
					resource.TestCheckResourceAttr("netgear_vlan.test", "untagged_ports.#", "0"),
				),
			},
		},
	})
}

func TestAccNetgearVlan_import(t *testing.T) {
	testAccPreCheck(t)

	id := accVlanID()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "netgear_vlan" "test" {
  vlan_id = %[1]s
  name    = "tf acc import"
}`, id),
			},
			{
				ResourceName:      "netgear_vlan.test",
				ImportState:       true,
				ImportStateId:     id,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccNetgearInterface_basic(t *testing.T) {
	testAccPreCheck(t)

	port := accPort()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "netgear_interface" "test" {
  port        = %[1]q
  description = "tf acc"
  mtu         = 9216
}`, port),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netgear_interface.test", "id", port),
					resource.TestCheckResourceAttr("netgear_interface.test", "description", "tf acc"),
					resource.TestCheckResourceAttr("netgear_interface.test", "mtu", "9216"),
					resource.TestCheckResourceAttrSet("netgear_interface.test", "admin_status"),
				),
			},
			// Clearing every managed setting leaves the port at its defaults,
			// which FASTPATH stops printing in the running config. The empty
			// re-plan this step asserts is what caught the resource being
			// dropped from state and recreated forever.
			{
				Config: fmt.Sprintf(`resource "netgear_interface" "test" {
  port = %[1]q
}`, port),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("netgear_interface.test", "description"),
					resource.TestCheckResourceAttr("netgear_interface.test", "enabled", "true"),
				),
			},
			{
				ResourceName:      "netgear_interface.test",
				ImportState:       true,
				ImportStateId:     port,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccNetgearLag_basic(t *testing.T) {
	testAccPreCheck(t)

	id, first, second := accLagID(), accPort(), accPort2()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "netgear_lag" "test" {
  lag_id  = %[1]s
  name    = "tf acc"
  members = [%[2]q, %[3]q]
}`, id, first, second),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netgear_lag.test", "id", id),
					resource.TestCheckResourceAttr("netgear_lag.test", "members.#", "2"),
					resource.TestCheckResourceAttr("netgear_lag.test", "interface_id", "3/"+id),
					// Static is the switch default and is never written to the
					// config, so a group left alone reads back as lacp.
					resource.TestCheckResourceAttr("netgear_lag.test", "mode", "lacp"),
				),
			},
			{
				Config: fmt.Sprintf(`resource "netgear_lag" "test" {
  lag_id  = %[1]s
  name    = "tf acc"
  members = [%[2]q]
}`, id, first),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netgear_lag.test", "members.#", "1"),
					resource.TestCheckTypeSetElemAttr("netgear_lag.test", "members.*", first),
				),
			},
		},
	})
}

func TestAccNetgearDataSources_read(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "netgear_vlans" "all" {}
data "netgear_interfaces" "all" {}
data "netgear_lags" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.netgear_interfaces.all", "id", "interfaces"),
					resource.TestCheckResourceAttrWith("data.netgear_interfaces.all", "interfaces.#", func(value string) error {
						if value == "0" {
							return fmt.Errorf("the switch reported no ports")
						}
						return nil
					}),
					// Ports are the slot form and LAG interfaces live in unit 3,
					// which netgear_lags reports instead.
					resource.TestMatchResourceAttr("data.netgear_interfaces.all", "interfaces.0.port", regexp.MustCompile(`^0/\d+$`)),
					resource.TestCheckResourceAttrSet("data.netgear_vlans.all", "vlans.#"),
					resource.TestCheckResourceAttrSet("data.netgear_lags.all", "lags.#"),
				),
			},
		},
	})
}
