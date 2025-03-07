package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"terraform-provider-secops/internal/query"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// The docs _insist_ that these should be snake_case, but the API responds in camelCase...
type ReferenceList struct {
	Name                  string                  `json:"name,omitempty"`
	DisplayName           string                  `json:"displayName,omitempty"`
	RevisionCreateTime    string                  `json:"revision,omitempty"`
	Description           string                  `json:"description,omitempty"`
	Entries               []ReferenceListEntry    `json:"entries,omitempty"`
	Rule                  []string                `json:"rule,omitempty"`
	SyntaxType            ReferenceListSyntaxType `json:"syntaxType,omitempty"`
	RuleAssociationsCount int                     `json:"ruleAssociationsCount,omitempty"`
	ScopeInfo             RefereneListScope       `json:"scopeInfo"`
}

type ReferenceListSyntaxType = string

var (
	SyntaxUnspecificed ReferenceListSyntaxType = "REFERENCE_LIST_SYNTAX_TYPE_UNSPECIFIED"
	SyntaxPlaintext    ReferenceListSyntaxType = "REFERENCE_LIST_SYNTAX_TYPE_PLAIN_TEXT_STRING"
	SyntaxRegex        ReferenceListSyntaxType = "REFERENCE_LIST_SYNTAX_TYPE_REGEX"
	SyntaxCIDR         ReferenceListSyntaxType = "REFERENCE_LIST_SYNTAX_TYPE_CIDR"
)

type ReferenceListCreateDTO struct {
	Description string                  `json:"description"`
	Entries     []ReferenceListEntry    `json:"entries"`
	SyntaxType  ReferenceListSyntaxType `json:"syntax_type"`
}

type ReferenceListEntry struct {
	Value string `json:"value,omitempty"`
}

type RefereneListScope struct {
	ReferenceListScopes struct {
		ScopeNames []string `json:"scope_names,omitempty"`
	} `json:"reference_list_scopes"`
}

func (c *Client) CreateReferenceList(ctx context.Context, name string, description string, entries []string) (*ReferenceList, error) {
	name = strings.ReplaceAll(name, "\"", "")
	url := fmt.Sprintf("%s/referenceLists?referenceListId=%s", c.instanceURL, name)

	ents := []ReferenceListEntry{}
	for _, e := range entries {
		e = strings.ReplaceAll(e, "\"", "")
		ents = append(ents, ReferenceListEntry{Value: e})
	}

	tflog.Debug(ctx, fmt.Sprintf("performing request to read reference list with name %s and %d entries", name, len(entries)))
	b := ReferenceListCreateDTO{
		Description: description,
		Entries:     ents,
		SyntaxType:  SyntaxUnspecificed,
	}
	return query.Query[ReferenceListCreateDTO, ReferenceList](ctx, http.MethodPost, url, b, query.WithBearer(c.token))
}

// removes any added "s, and returns the substring after the last /.
func getDisplayNameSegment(name string) string {
	name = strings.ReplaceAll(name, "\"", "")
	segments := strings.Split(name, "/")
	return segments[len(segments)-1]
}

func (c *Client) GetReferenceList(ctx context.Context, displayName string) (*ReferenceList, error) {
	name := getDisplayNameSegment(displayName)
	url := fmt.Sprintf("%s/referenceLists/%s", c.instanceURL, name)

	refList, err := query.Get[ReferenceList](ctx, url, query.WithBearer(c.token))
	if err != nil {
		return nil, err
	}
	tflog.Debug(ctx, "got reference list", map[string]any{
		"entiresCount": len(refList.Entries),
	})

	return refList, nil
}

// Subset of ReferenceList, with updatable fields
type refListUpdateDTO struct {
	Entries     []ReferenceListEntry `json:"entries"`
	DisplayName string               `json:"name"`
	Description string               `json:"description"`
}

func unquote(s string) string {
	return strings.ReplaceAll(s, "\"", "")
}

func (c *Client) UpdateReferenceList(ctx context.Context, displayName string, entries []string, description string) (*ReferenceList, error) {
	displayName = getDisplayNameSegment(displayName)
	tflog.Debug(ctx, "updating reference list", map[string]any{
		"displayName": displayName,
		"entries":     entries,
	})
	url := fmt.Sprintf("%s/referenceLists/%s?updateMask=*", c.instanceURL, displayName)

	ents := []ReferenceListEntry{}
	for _, e := range entries {
		ents = append(ents, ReferenceListEntry{Value: unquote(e)})
	}

	update := refListUpdateDTO{Entries: ents, DisplayName: unquote(displayName), Description: unquote(description)}

	return query.Query[refListUpdateDTO, ReferenceList](ctx, http.MethodPatch, url, update, query.WithBearer(c.token))
}
