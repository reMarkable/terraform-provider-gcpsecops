package provider

// This file exists to hold functions that reduce the boilerplate required to write plan/state calls

import (
	"context"
	"fmt"
	"terraform-provider-secops/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func toStringSlice[T fmt.Stringer](l []T) []string {
	var res []string
	for _, e := range l {
		res = append(res, e.String())
	}

	return res
}

func toListValue(ents []client.ReferenceListEntry) (basetypes.ListValue, diag.Diagnostics) {
	vals := []attr.Value{}
	for _, e := range ents {
		vals = append(vals, types.StringValue(e.Value))
	}

	return types.ListValue(types.StringType, vals)
}

type Getter interface {
	Get(context.Context, any) diag.Diagnostics
}
