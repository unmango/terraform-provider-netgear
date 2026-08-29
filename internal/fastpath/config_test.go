package fastpath_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

const sampleConfig = `!Current Configuration:
!
!System Description "GS724Tv4"
!
configure
vlan database
vlan 10,20
vlan name 10 "mgmt"
vlan name 20 "voice"
vlan routing 10
exit
interface 0/1
description "workstation"
vlan pvid 10
vlan participation include 10
exit
interface 0/2
shutdown
mtu 9216
speed 1000 full
vlan participation include 20
exit
interface 0/24
description 'uplink'
vlan participation include 10,20
vlan tagging 10
vlan tagging 20
exit
`

var _ = Describe("ParseRunningConfig", func() {
	var config *fastpath.RunningConfig

	BeforeEach(func() {
		config = fastpath.ParseRunningConfig(sampleConfig)
	})

	It("should read the vlan database", func() {
		Expect(config.VLANs).To(HaveLen(2))

		mgmt, ok := config.VLAN(10)

		Expect(ok).To(BeTrue())
		Expect(mgmt.Name).To(Equal("mgmt"))
		Expect(mgmt.Routing).To(BeTrue())
	})

	It("should report a vlan the switch does not have", func() {
		_, ok := config.VLAN(99)

		Expect(ok).To(BeFalse())
	})

	It("should fold interface membership onto the vlan", func() {
		mgmt, _ := config.VLAN(10)

		Expect(mgmt.Untagged).To(ConsistOf("0/1"))
		Expect(mgmt.Tagged).To(ConsistOf("0/24"))
	})

	It("should expand comma separated participation", func() {
		voice, _ := config.VLAN(20)

		Expect(voice.Untagged).To(ConsistOf("0/2"))
		Expect(voice.Tagged).To(ConsistOf("0/24"))
	})

	It("should read port settings", func() {
		port, ok := config.Interface("0/2")

		Expect(ok).To(BeTrue())
		Expect(port.Shutdown).To(BeTrue())
		Expect(port.MTU).To(BeEquivalentTo(9216))
		Expect(port.Speed).To(Equal("1000-full"))
	})

	It("should unquote descriptions in either quote style", func() {
		workstation, _ := config.Interface("0/1")
		uplink, _ := config.Interface("0/24")

		Expect(workstation.Description).To(Equal("workstation"))
		Expect(uplink.Description).To(Equal("uplink"))
	})

	It("should read the pvid", func() {
		workstation, _ := config.Interface("0/1")

		Expect(workstation.PVID).To(BeEquivalentTo(10))
	})

	It("should expand hyphenated id ranges", func() {
		config := fastpath.ParseRunningConfig("vlan database\nvlan 10-12\nexit\n")

		Expect(config.VLANs).To(HaveLen(3))
		Expect(config.VLANs).To(HaveKey(int64(11)))
	})

	It("should ignore an empty config", func() {
		config := fastpath.ParseRunningConfig("")

		Expect(config.VLANs).To(BeEmpty())
		Expect(config.Interfaces).To(BeEmpty())
	})
})
