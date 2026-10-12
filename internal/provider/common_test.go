package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestRequiresReplaceIfKnown pins the case it exists for: an imported product
// holds no title, because RevenueCat never returns one, and naming the title in
// configuration must adopt it rather than destroy a live product.
func TestRequiresReplaceIfKnown(t *testing.T) {
	cases := []struct {
		name  string
		state types.String
		plan  types.String
		want  bool
	}{
		{"an imported object takes the configured value", types.StringNull(), types.StringValue("Pro Monthly"), false},
		{"a known value that changes replaces", types.StringValue("Pro Monthly"), types.StringValue("Pro, monthly"), true},
		{"a value left alone does not", types.StringValue("Pro Monthly"), types.StringValue("Pro Monthly"), false},
	}

	// The framework skips a replace on create and destroy before asking the
	// modifier, so a resource that exists in both plan and state is the case.
	existing := tfsdk.State{Raw: tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})}
	planned := tfsdk.Plan{Raw: tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue: tc.state,
				PlanValue:  tc.plan,
				State:      existing,
				Plan:       planned,
			}
			var resp planmodifier.StringResponse
			resp.PlanValue = tc.plan
			requiresReplaceIfKnown().PlanModifyString(context.Background(), req, &resp)
			if resp.RequiresReplace != tc.want {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, tc.want)
			}
		})
	}
}
