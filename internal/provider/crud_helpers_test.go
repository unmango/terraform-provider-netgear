package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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

// dataSchemaOf populates a data source's schema and asserts it produced no
// diagnostics.
func dataSchemaOf(ctx context.Context, d datasource.DataSource) dschema.Schema {
	GinkgoHelper()

	resp := &datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, resp)

	Expect(resp.Diagnostics.HasError()).To(BeFalse(), "%v", resp.Diagnostics)

	return resp.Schema
}

// configFor builds the configuration a data source reads its lookup key from.
func configFor(ctx context.Context, d datasource.DataSource, model any) tfsdk.Config {
	GinkgoHelper()

	schema := dataSchemaOf(ctx, d)

	// tfsdk.Config has no Set, so the value is built through a state and handed
	// over: both are the same object type behind the same schema.
	state := tfsdk.State{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
	}

	Expect(state.Set(ctx, model).HasError()).To(BeFalse())

	return tfsdk.Config{Schema: schema, Raw: state.Raw}
}

// blankDataState is the empty state a data source Read writes into.
func blankDataState(ctx context.Context, d datasource.DataSource) tfsdk.State {
	schema := dataSchemaOf(ctx, d)

	return tfsdk.State{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
	}
}
