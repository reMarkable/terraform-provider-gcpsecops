package provider

import (
	"context"
	"fmt"
	"terraform-provider-secops/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &RuleResource{}
	_ resource.ResourceWithImportState = &RuleResource{}
	_ resource.ResourceWithConfigure   = &RuleResource{}
)

func NewRuleResource() resource.Resource {
	return &RuleResource{}
}

// RuleResource defines the resource implementation.
type RuleResource struct {
	client *client.Client
}

func (r *RuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *hashicups.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Create implements resource.ResourceWithImportState.
func (r *RuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RuleDataModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var text string

	diags = req.Plan.GetAttribute(ctx, path.Root("text"), &text)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.CreateRule(ctx, client.Rule{
		SecopsRuleDTO: client.SecopsRuleDTO{
			Text: text,
		},
		Deployment: client.RuleDeploymentDTO{
			Alerting: plan.Alerting.ValueBool(),
			Enabled:  plan.Enabled.ValueBool(),
			Archived: plan.Archived.ValueBool(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Rule",
			"Could not create Rule: "+err.Error(),
		)
		return
	}

	plan.Name = types.StringValue(rule.Name)
	plan.Text = types.StringValue(rule.Text)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete implements resource.ResourceWithImportState.
func (r *RuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RuleDataModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRule(ctx, data.Name.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Rule",
			"Could not delete Rule with name "+data.Name.String()+": "+err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ImportState implements resource.ResourceWithImportState.
func (r *RuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// Metadata implements resource.ResourceWithImportState.
func (r *RuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

// Read implements resource.ResourceWithImportState.
func (r *RuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RuleDataModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetRule(ctx, data.Name.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Rule",
			"Could not read Rule with name "+data.Name.String()+": "+err.Error(),
		)
		return
	}

	data.Name = types.StringValue(rule.Name)
	data.Text = types.StringValue(rule.Text)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type RuleDataModel struct {
	Name     types.String `tfsdk:"name"`
	Text     types.String `tfsdk:"text"`
	Archived types.Bool   `tfsdk:"archived"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Alerting types.Bool   `tfsdk:"alerting"`
}

// Schema implements resource.ResourceWithImportState.
func (r *RuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "An internal ID used by the Google Secops API and terraform",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"text": schema.StringAttribute{
				Required: true,
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"archived": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"alerting": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Update implements resource.ResourceWithImportState.
func (r *RuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RuleDataModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "updating rule", map[string]any{
		"name":     plan.Name.ValueString(),
		"archived": plan.Archived.ValueBool(),
		"alerting": plan.Alerting.ValueBool(),
		"enabled":  plan.Enabled.ValueBool(),
	})

	updateMask := client.Rule{
		SecopsRuleDTO: client.SecopsRuleDTO{
			Name: plan.Name.ValueString(),
			Text: plan.Text.ValueString(),
		},
		Deployment: client.RuleDeploymentDTO{
			Archived: plan.Archived.ValueBool(),
			Enabled:  plan.Enabled.ValueBool(),
			Alerting: plan.Alerting.ValueBool(),
		},
	}

	updatedRule, err := r.client.UpdateRule(ctx, updateMask)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Rule",
			"Could not update Rule with name "+plan.Name.String()+": "+err.Error(),
		)
		return
	}
	plan.Name = types.StringValue(updatedRule.Name)
	plan.Text = types.StringValue(updatedRule.Text)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
