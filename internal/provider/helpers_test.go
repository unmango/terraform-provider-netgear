package provider_test

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// resourceSchema populates a resource's schema and asserts it produced no diagnostics.
func resourceSchema(ctx context.Context, r resource.Resource) rschema.Schema {
	GinkgoHelper()

	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)

	Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

	return resp.Schema
}
