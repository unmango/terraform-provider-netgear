package fastpath_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/UnstoppableMango/terraform-provider-netgear/internal/fastpath"
)

func clientFor(sw *fakeSwitch, flow fastpath.Flow) *fastpath.Client {
	GinkgoHelper()

	client, err := fastpath.NewWithDialer(fastpath.Config{
		Username:       "admin",
		Password:       "secret",
		EnablePassword: "",
		Flow:           flow,
		Timeout:        5 * time.Second,
	}, sw.dialer())

	Expect(err).NotTo(HaveOccurred())

	return client
}

var _ = Describe("Client", func() {
	It("should require a host", func() {
		_, err := fastpath.New(fastpath.Config{})

		Expect(err).To(MatchError(ContainSubstring("host is required")))
	})

	It("should default the port to 60000", func() {
		client, err := fastpath.New(fastpath.Config{Host: "192.0.2.10"})

		Expect(err).NotTo(HaveOccurred())
		Expect(client).NotTo(BeNil())
	})

	It("should report the address it could not reach", func(ctx SpecContext) {
		client, err := fastpath.New(fastpath.Config{
			Host:     "127.0.0.1",
			Port:     1,
			Password: "secret",
			Timeout:  2 * time.Second,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = client.Run(ctx, "show version")

		Expect(err).To(MatchError(ContainSubstring("127.0.0.1:1")))
	})

	Describe("newer firmware", func() {
		var sw *fakeSwitch

		BeforeEach(func() {
			sw = &fakeSwitch{
				model: "GS724Tv4",
				responses: map[string]string{
					"show running-config": "!Current Configuration:\r\nvlan database\r\nvlan 10\r\nexit",
				},
			}
		})

		It("should log in and disable pagination", func(ctx SpecContext) {
			client := clientFor(sw, fastpath.FlowAuto)

			_, err := client.Run(ctx, "show version")

			Expect(err).NotTo(HaveOccurred())
			Expect(sw.received()).To(HaveExactElements("terminal length 0", "show version"))
		})

		It("should return command output without the echo or prompt", func(ctx SpecContext) {
			client := clientFor(sw, fastpath.FlowAuto)

			out, err := client.RunningConfig(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(HavePrefix("!Current Configuration:"))
			Expect(out).To(ContainSubstring("vlan 10"))
			Expect(out).NotTo(ContainSubstring("GS724Tv4"))
		})

		It("should run commands in order", func(ctx SpecContext) {
			client := clientFor(sw, fastpath.FlowAuto)

			_, err := client.Run(ctx, "configure", "vlan database", "vlan 10")

			Expect(err).NotTo(HaveOccurred())
			Expect(sw.received()).To(HaveExactElements(
				"terminal length 0", "configure", "vlan database", "vlan 10",
			))
		})

		It("should stop at the first rejected command", func(ctx SpecContext) {
			sw.responses["vlan 4095"] = "% Invalid input detected at '^' marker."
			client := clientFor(sw, fastpath.FlowAuto)

			_, err := client.Run(ctx, "vlan 4095", "vlan 10")

			Expect(err).To(MatchError(ContainSubstring("Invalid input detected")))
			Expect(sw.received()).NotTo(ContainElement("vlan 10"))
		})

		It("should save the configuration", func(ctx SpecContext) {
			client := clientFor(sw, fastpath.FlowAuto)

			err := client.Save(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(sw.received()).To(ContainElement(saveCommand))
		})

		It("should answer the save confirmation prompt", func(ctx SpecContext) {
			sw.confirmSave = true
			client := clientFor(sw, fastpath.FlowAuto)

			err := client.Save(ctx)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should refuse telnet options", func(ctx SpecContext) {
			sw.negotiate = true
			client := clientFor(sw, fastpath.FlowAuto)

			out, err := client.RunningConfig(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("vlan 10"))
		})
	})

	Describe("older firmware", func() {
		var sw *fakeSwitch

		BeforeEach(func() {
			sw = &fakeSwitch{
				model:     "Broadcom FASTPATH Switching",
				legacy:    true,
				responses: map[string]string{"show version": "Machine Model... GS748Tv4"},
			}
		})

		It("should answer the enable password prompt", func(ctx SpecContext) {
			client := clientFor(sw, fastpath.FlowAuto)

			out, err := client.Run(ctx, "show version")

			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("GS748Tv4"))
		})

		It("should reject the legacy enable prompt when the modern flow is forced", func(ctx SpecContext) {
			client := clientFor(sw, fastpath.FlowModern)

			_, err := client.Run(ctx, "show version")

			Expect(err).To(MatchError(ContainSubstring("enable password")))
		})
	})
})
