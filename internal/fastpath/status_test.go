package fastpath_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/unmango/terraform-provider-netgear/internal/fastpath"
)

// showPort is `show port all` as a GS724Tv4 running 6.3.1.19 prints it.
const showPort = `                 Admin     Physical   Physical   Link   Link    LACP   Flow
Intf      Type   Mode      Mode       Status     Status Trap    Mode   Mode
--------- ------ --------- ---------- ---------- ------ ------- ------ -------
g7               Enable    Auto       1000 Full  Up     Enable  Enable Disable
g8               Enable    Auto                  Down   Enable  Enable Disable
g13              Disable   Auto                  Down   Enable  Enable Disable
lag 1            Enable                          Down   Enable  N/A    Disable
`

// showInterfacesStatus is `show interfaces status`, which has no admin column and
// names ports in the slot form.
const showInterfacesStatus = `                                         Link    Physical    Physical    Media               Flow Control
Port       Name                          State   Mode        Status      Type                Status
---------  ----------------------------  ------  ----------  ----------  ------------------  ------------
0/1                                      Up      Auto        1000 Full   10/100/1000-BaseTx  Inactive
0/4                                      Down    Auto                    10/100/1000-BaseTx  Inactive
3/1                                      Down

Flow Control:Disabled
`

var _ = Describe("ParsePortStatus", func() {
	Describe("show port", func() {
		var statuses map[string]fastpath.PortStatus

		BeforeEach(func() {
			statuses = fastpath.ParsePortStatus(showPort)
		})

		It("should key ports by the slot form", func() {
			Expect(statuses).To(HaveKey("0/7"))
			Expect(statuses).NotTo(HaveKey("g7"))
		})

		It("should read an enabled port that is up", func() {
			Expect(statuses["0/7"]).To(Equal(fastpath.PortStatus{
				AdminStatus: "enable",
				LinkStatus:  "up",
			}))
		})

		It("should not mistake a later column for the admin mode", func() {
			Expect(statuses["0/13"].AdminStatus).To(Equal("disable"))
		})

		It("should read a link that is down", func() {
			Expect(statuses["0/8"].LinkStatus).To(Equal("down"))
		})

		It("should key a lag by its slot id", func() {
			Expect(statuses).To(HaveKey("3/1"))
			Expect(statuses["3/1"].LinkStatus).To(Equal("down"))
		})

		It("should ignore the header and rule rows", func() {
			Expect(statuses).To(HaveLen(4))
		})
	})

	Describe("show interfaces status", func() {
		var statuses map[string]fastpath.PortStatus

		BeforeEach(func() {
			statuses = fastpath.ParsePortStatus(showInterfacesStatus)
		})

		It("should read the link state without an admin column", func() {
			Expect(statuses["0/1"]).To(Equal(fastpath.PortStatus{LinkStatus: "up"}))
			Expect(statuses["0/4"].LinkStatus).To(Equal("down"))
		})

		It("should ignore the trailing summary line", func() {
			Expect(statuses).To(HaveLen(3))
		})
	})

	It("should survive a truncated row", func() {
		statuses := fastpath.ParsePortStatus("0/5\n")

		Expect(statuses).To(HaveKey("0/5"))
		Expect(statuses["0/5"].Known()).To(BeFalse())
	})

	It("should report nothing for empty output", func() {
		Expect(fastpath.ParsePortStatus("")).To(BeEmpty())
	})
})

var _ = Describe("NormalizePort", func() {
	DescribeTable("should convert the short form the switch prints",
		func(id, expected string) {
			Expect(fastpath.NormalizePort(id)).To(Equal(expected))
		},
		Entry("a gigabit port", "g7", "0/7"),
		Entry("a lag with a space", "lag 1", "3/1"),
		Entry("a lag without a space", "lag2", "3/2"),
		Entry("a slot id already", "0/7", "0/7"),
		Entry("a lag slot id already", "3/1", "3/1"),
		Entry("something unrecognized", "eth0", "eth0"),
	)
})
