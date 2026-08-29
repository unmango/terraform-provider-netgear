package fastpath_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

const sampleStatus = `                                      Link    Physical    Physical    Media       Flow
Intf       Type    Admin Mode        State   Mode        Status      Type        Control
---------- ------  ----------------- ------- ----------- ----------- ----------  --------
0/1                Enable            Up      Auto        1000 Full   Copper      Inactive
0/2                Disable           Down    Auto                    Copper      Inactive
0/24               Enable            Down    Auto                    Copper      Inactive
3/1                Enable            Up
`

var _ = Describe("ParseInterfaceStatus", func() {
	var statuses map[string]fastpath.PortStatus

	BeforeEach(func() {
		statuses = fastpath.ParseInterfaceStatus(sampleStatus)
	})

	It("should read every port row", func() {
		Expect(statuses).To(HaveLen(4))
	})

	It("should normalize an enabled port that is up", func() {
		Expect(statuses["0/1"]).To(Equal(fastpath.PortStatus{
			AdminStatus: "enable",
			LinkStatus:  "up",
		}))
	})

	It("should read a disabled port", func() {
		Expect(statuses["0/2"]).To(Equal(fastpath.PortStatus{
			AdminStatus: "disable",
			LinkStatus:  "down",
		}))
	})

	It("should read a lag interface", func() {
		Expect(statuses).To(HaveKey("3/1"))
	})

	It("should ignore the header and rule rows", func() {
		Expect(statuses).NotTo(HaveKey("Intf"))
		Expect(statuses).NotTo(HaveKey("----------"))
	})

	It("should survive a truncated row", func() {
		statuses := fastpath.ParseInterfaceStatus("0/5\n")

		Expect(statuses).To(HaveKey("0/5"))
		Expect(statuses["0/5"].Known()).To(BeFalse())
	})

	It("should report nothing for empty output", func() {
		Expect(fastpath.ParseInterfaceStatus("")).To(BeEmpty())
	})

	It("should not mistake a media column for the link state", func() {
		statuses := fastpath.ParseInterfaceStatus("0/9  Enable  Down  Auto  Up-Down  Copper\n")

		Expect(statuses["0/9"].LinkStatus).To(Equal("down"))
	})
})
