package fastpath_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

var _ = Describe("Ports", func() {
	DescribeTable("NormalizePort",
		func(id, expected string) {
			Expect(fastpath.NormalizePort(id)).To(Equal(expected))
		},
		Entry("short port", "g7", "0/7"),
		Entry("short lag", "lag 1", "3/1"),
		Entry("slot form", "0/7", "0/7"),
		Entry("unknown spelling", "eth0", "eth0"),
	)

	DescribeTable("IsLAG",
		func(id string, expected bool) {
			Expect(fastpath.IsLAG(id)).To(Equal(expected))
		},
		Entry("short lag", "lag 1", true),
		Entry("slot lag", "3/1", true),
		Entry("short port", "g7", false),
		Entry("slot port", "0/7", false),
	)
})
