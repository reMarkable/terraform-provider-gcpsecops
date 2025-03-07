package provider

import (
	"context"
	"fmt"
	"terraform-provider-secops/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &ReferenceListResource{}
	_ resource.ResourceWithImportState = &ReferenceListResource{}
	_ resource.ResourceWithConfigure   = &ReferenceListResource{}
)

type ReferenceListDataModel struct {
	Name        types.String `tfsdk:"name"`
	Entries     types.List   `tfsdk:"entries"`
	Description types.String `tfsdk:"description"`
	DisplayName types.String `tfsdk:"display_name"`
}

type ReferenceListResource struct {
	client *client.Client
}

func NewReferenceListResource() resource.Resource {
	return &ReferenceListResource{}
}

func (r *ReferenceListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Calling Create", map[string]any{})
	var plan ReferenceListDataModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refList, err := r.client.CreateReferenceList(ctx, plan.DisplayName.String(), plan.Description.String(), toStringSlice(plan.Entries.Elements()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating reference list",
			"Could not read reference list with name "+plan.DisplayName.String()+": "+err.Error(),
		)
		return
	}

	entries, _ := toListValue(refList.Entries)
	plan.Entries = entries
	plan.Name = types.StringValue(refList.Name)
	plan.DisplayName = types.StringValue(refList.DisplayName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReferenceListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Calling Read", map[string]any{})
	var state ReferenceListDataModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refList, err := r.client.GetReferenceList(ctx, state.Name.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading reference list",
			"Could not read reference list with name "+state.Name.String()+": "+err.Error(),
		)
		return
	}

	entries, diags := toListValue(refList.Entries)
	resp.Diagnostics.Append(diags...)

	state.Entries = entries
	state.DisplayName = types.StringValue(refList.DisplayName)
	state.Name = types.StringValue(refList.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ReferenceListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Debug(ctx, "Calling Update", map[string]any{})
	var plan ReferenceListDataModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	refList, err := r.client.UpdateReferenceList(ctx, plan.DisplayName.String(), toStringSlice(plan.Entries.Elements()), plan.Description.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error update reference list",
			"Could not update reference list with name "+plan.DisplayName.String()+": "+err.Error(),
		)
		return
	}

	ents, diags := toListValue(refList.Entries)

	tflog.Debug(ctx, "entries", map[string]any{
		"name":                   refList.Name,
		"entries":                refList.Entries,
		"entriesCount":           len(refList.Entries),
		"entriesStrValList":      ents,
		"enrtiesStrValListCount": len(ents.Elements()),
	})
	resp.Diagnostics.Append(diags...)
	plan.Name = types.StringValue(refList.Name)
	plan.DisplayName = types.StringValue(refList.DisplayName)
	plan.Description = types.StringValue(refList.Description)
	plan.Entries = ents

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete isn't implemented in the GCP SecOps API, and as such this call only removes the object from state.
func (r *ReferenceListResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Warn(ctx, "You're trying to delete a reference list. This isn't supported in the GCP SecOps API. Performing this action only removes the list from the terraform state, and does not remove it from GCP SecOps")
	var state ReferenceListDataModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// There's no API support for deleting reference lists
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ReferenceListResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"display_name": schema.StringAttribute{
				Description: "Name of the reference list",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				Description: "Name of the reference list. This is used as an internal identifier for terraform",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},

			"description": schema.StringAttribute{
				Description: "Description of the list",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"entries": schema.ListAttribute{
				Description: "List of strings to include in the list",
				ElementType: types.StringType,
				Required:    true,
			},
		},
	}
}

func (r *ReferenceListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *ReferenceListResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reference_list"
}

func (r *ReferenceListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}
