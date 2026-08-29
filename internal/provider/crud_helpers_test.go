package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// schemaOf populates a resource's schema and asserts it produced no diagnostics.
func schemaOf(ctx context.Context, r resource.Resource) rschema.Schema {
	GinkgoHelper()

	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)

	Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

	return resp.Schema
}

// nullObject is the value a fresh plan or state starts from.
func nullObject(ctx context.Context, schema rschema.Schema) tftypes.Value {
	return tftypes.NewValue(schema.Type().TerraformType(ctx), nil)
}

// planFor builds a plan holding model, for driving a resource's CRUD directly.
func planFor(ctx context.Context, r resource.Resource, model any) tfsdk.Plan {
	GinkgoHelper()

	schema := schemaOf(ctx, r)
	plan := tfsdk.Plan{Schema: schema, Raw: nullObject(ctx, schema)}

	Expect(plan.Set(ctx, model).HasError()).To(BeFalse())

	return plan
}

// stateFor builds a state holding model.
func stateFor(ctx context.Context, r resource.Resource, model any) tfsdk.State {
	GinkgoHelper()

	schema := schemaOf(ctx, r)
	state := tfsdk.State{Schema: schema, Raw: nullObject(ctx, schema)}

	Expect(state.Set(ctx, model).HasError()).To(BeFalse())

	return state
}

// blankState is the empty state a Create writes into.
func blankState(ctx context.Context, r resource.Resource) tfsdk.State {
	schema := schemaOf(ctx, r)

	return tfsdk.State{Schema: schema, Raw: nullObject(ctx, schema)}
}

// readState reads a model back out of a state.
func readState[T any](ctx context.Context, state tfsdk.State) T {
	GinkgoHelper()

	var model T

	Expect(state.Get(ctx, &model).HasError()).To(BeFalse())

	return model
}

// testSwitch is a resource's dependency wired to a fake client.
func testSwitch(client *fakeClient) *switchData {
	return &switchData{client: client, saveConfig: true}
}
