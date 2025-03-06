package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"terraform-provider-secops/internal/query"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type Severity struct {
	DisplayName string `json:",omitempty"`
}

type RuleDeploymentDTO struct {
	Name                      string   `json:"name,omitempty"`
	Enabled                   bool     `json:"enabled"`
	Alerting                  bool     `json:"alerting"`
	Archived                  bool     `json:"archived"`
	ArchiveTime               string   `json:"archive_time,omitempty"`
	RunFrequency              string   `json:"run_frequency,omitempty"`
	ExecutionState            string   `json:"execution_state,omitempty"`
	ProducerRules             []string `json:"producer_rules,omitempty"`
	ConsumerRules             []string `json:"consumer_rules,omitempty"`
	LastAlertStatusChangeTime string   `json:"last_alert_status_change_time,omitempty"`
}

type SecopsRuleDTO struct {
	Name                         string            `json:"name,omitempty"`
	DisplayName                  string            `json:"displayName,omitempty"`
	RevisionID                   string            `json:"revisionId,omitempty"`
	Text                         string            `json:"text,omitempty"`
	Author                       string            `json:"author,omitempty"`
	Severity                     Severity          `json:"severity,omitempty"`
	Metadata                     map[string]string `json:"metadata,omitempty"`
	CreateTime                   string            `json:"createTime,omitempty"`
	RevisionCreateTime           string            `json:"revisionCreateTime,omitempty"`
	CompilationState             string            `json:"compilationState,omitempty"`
	ReferenceLists               []string          `json:"referenceLists,omitempty"`
	RuleType                     string            `json:"ruleType,omitempty"`
	AllowedRunFrequencies        []string          `json:"allowedRunFrequencies,omitempty"`
	Etag                         string            `json:"etag,omitempty"`
	Scope                        string            `json:"scope,omitempty"`
	CompilationDiagnostics       []map[string]any  `json:"compilationDiagnostics,omitempty"`
	NearRealTimeLiveRuleEligible bool              `json:"nearRealTimeLiveRuleEligible,omitempty"`
	DataTables                   []string          `json:"dataTables,omitempty"`
}

func NewRule(text string) *Rule {
	return &Rule{
		SecopsRuleDTO: SecopsRuleDTO{
			Text: text,
		},
	}
}

type Rule struct {
	SecopsRuleDTO
	// Deployment is the only field not populated by the call to the rules API. It gets populated by the client, via the ruleDeployments API
	// It can also be used by the UpdateRule call to move betwen activation states, or set for the CreateRule call to override defaults
	Deployment RuleDeploymentDTO `json:"deployment,omitempty"`
}

func (c *Client) UpdateRule(ctx context.Context, update Rule) (*Rule, error) {
	ruleNameShort := strings.ReplaceAll(update.Name, "\"", "")
	ruleNameShort = strings.TrimPrefix(ruleNameShort, "projects/826325533751/locations/eu/instances/87b64822-b99b-453e-a4bc-ab380e798f87/rules/")

	url := fmt.Sprintf("%s/rules/%s", c.instanceURL, ruleNameShort)

	// If the deployment update is to archive the rule, this must be done before anything else
	if update.Deployment.Archived {
		if update.Deployment.Enabled {
			return nil, fmt.Errorf("can not archive rule that's marked as enabled. Must set set `enabled = false` alongside `archived = true`")
		}

		if update.Deployment.Alerting {
			tflog.Warn(ctx, "Call to archive rule has `alerting = true`. GCP forces this parameter to be set to `false` as part of archiving. This will cause drift from your config")
		}

		r, err := c.updateDeployment(ctx, update.SecopsRuleDTO.Name, update.Deployment)
		if err != nil {
			return nil, err
		}

		rr := Rule{
			SecopsRuleDTO: update.SecopsRuleDTO,
			Deployment:    *r,
		}

		return &rr, nil
	}

	_, err := c.updateDeployment(ctx, update.SecopsRuleDTO.Name, update.Deployment)
	if err != nil {
		return nil, err
	}

	tflog.Debug(ctx, "Updating rule", map[string]any{
		"rule_name":       update.Name,
		"rule_name_short": ruleNameShort,
		"url":             url,
	})

	b, err := json.Marshal(&update.SecopsRuleDTO)
	if err != nil {
		return nil, fmt.Errorf("%w failed to marshal json", err)
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("%w failed to construct request", err)
	}

	bb, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%w failed to perform request", err)
	}

	var newRule Rule
	err = json.Unmarshal(bb, &newRule)
	if err != nil {
		return nil, fmt.Errorf("%w failed to unmarshal server response", err)
	}

	_, err = c.updateDeployment(ctx, newRule.Name, update.Deployment)
	if err != nil {
		return nil, err
	}

	return &newRule, nil
}

func (c *Client) updateDeployment(ctx context.Context, ruleName string, newDeployment RuleDeploymentDTO) (*RuleDeploymentDTO, error) {
	ruleNameShort := strings.ReplaceAll(ruleName, "\"", "")
	ruleNameShort = strings.TrimPrefix(ruleNameShort, "projects/826325533751/locations/eu/instances/87b64822-b99b-453e-a4bc-ab380e798f87/rules/")

	currentDeploy, err := c.getDeployment(ctx, ruleName)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/rules/%s/deployment?updateMask=enabled,alerting", c.instanceURL, ruleNameShort)

	tflog.Debug(ctx, "Updating deployment", map[string]any{
		"rule_name":       ruleName,
		"rule_name_short": ruleNameShort,
		"url":             url,
	})

	b, err := json.Marshal(newDeployment)
	if err != nil {
		return nil, err
	}

	// Archived.
	//
	// This handles the case where we need to change the archive status of a rule. This call is identical to the one below, but we must resolve the
	// archived state before all others, due to how the GCP SecOps API works. Updating all the fields at once causes errors, so this order must be preserved
	if newDeployment.Archived != currentDeploy.Archived {
		urlArchived := fmt.Sprintf("%s/rules/%s/deployment?updateMask=archived", c.instanceURL, ruleNameShort)

		deployment, err := query.PatchM(ctx, urlArchived, newDeployment)
		if err != nil {
			return nil, err
		}

		// The newly updated deployment has set the state to archived. This means we can no longer do any other changes
		if newDeployment.Archived {
			return deployment, nil
		}
	}

	// Enabled and alerting
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	var e GCPErrorMessage
	bb, err := c.doRequest(ctx, req)
	// There are certain error states in the GCP SecOps API that has no practical significance to us. The primary is if a field is already set to
	// the value that it's being updated to. This can happen for convenience reasons, and the alternative involves a tonne more boilerplate.
	// So we just ignore those cases
	if err != nil && bb != nil {
		if jerr := json.Unmarshal(bb, &e); jerr == nil {
			// This is the case where enabled or alerting already is true. Do not ask me why its named that, it just is
			if e.Error.Status == "ALREADY_EXISTS" {
				err = nil
			}
			for _, d := range e.Error.Details {
				// This is the case where enabled already is false.
				// Why these two opposite cases of a binary check causes error messages in different parts of the JSON I also do not know
				if d.Reason == "RULE_ALREADY_DISABLED" {
					err = nil
				}
			}
		}
	}

	if err != nil {
		return nil, err
	}

	var resultDeployment RuleDeploymentDTO
	err = json.Unmarshal(bb, &resultDeployment)
	if err != nil {
		return nil, err
	}

	return &resultDeployment, nil
}

type GCPErrorMessage struct {
	Error GCPError `json:"error"`
}

type GCPError struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Status  string           `json:"status"`
	Details []GCPErrorDetail `json:"details"`
}

type GCPErrorDetail struct {
	Type   string `json:"@type"`
	Reason string `json:"reason"`
	Domain string `json:"domain"`
}

func (c *Client) getDeployment(ctx context.Context, ruleName string) (*RuleDeploymentDTO, error) {
	ruleNameShort := strings.ReplaceAll(ruleName, "\"", "")
	ruleNameShort = strings.TrimPrefix(ruleNameShort, "projects/826325533751/locations/eu/instances/87b64822-b99b-453e-a4bc-ab380e798f87/rules/")

	url := fmt.Sprintf("%s/rules/%s/deployment", c.instanceURL, ruleNameShort)

	tflog.Debug(ctx, "Getting deployment", map[string]any{
		"rule_name":       ruleName,
		"rule_name_short": ruleNameShort,
		"url":             url,
	})

	return query.Get[RuleDeploymentDTO](ctx, url, query.WithBearer(c.token))
}

func (c *Client) DeleteRule(ctx context.Context, ruleName string) error {
	ruleNameShort := strings.ReplaceAll(ruleName, "\"", "")
	ruleNameShort = strings.TrimPrefix(ruleNameShort, "projects/826325533751/locations/eu/instances/87b64822-b99b-453e-a4bc-ab380e798f87/rules/")

	url := fmt.Sprintf("%s/rules/%s", c.instanceURL, ruleNameShort)

	tflog.Debug(ctx, "Deleting rule", map[string]any{
		"rule_name":       ruleName,
		"rule_name_short": ruleNameShort,
		"url":             url,
	})
	return query.Delete(ctx, url, query.WithBearer(c.token))
}

func (c *Client) GetRule(ctx context.Context, ruleName string) (*Rule, error) {
	ruleNameShort := strings.ReplaceAll(ruleName, "\"", "")
	ruleNameShort = strings.TrimPrefix(ruleNameShort, "projects/826325533751/locations/eu/instances/87b64822-b99b-453e-a4bc-ab380e798f87/rules/")

	url := fmt.Sprintf("%s/rules/%s", c.instanceURL, ruleNameShort)

	tflog.Debug(ctx, "Getting rule", map[string]any{
		"rule_name":       ruleName,
		"rule_name_short": ruleNameShort,
		"url":             url,
	})

	return query.Get[Rule](ctx, url, query.WithBearer(c.token))
}

func (c *Client) CreateRule(ctx context.Context, r Rule) (*Rule, error) {
	url := fmt.Sprintf("%s/rules", c.instanceURL)

	var rule Rule
	ruledto, err := query.PostM(ctx, url, r.SecopsRuleDTO, query.WithBearer(c.token))
	if err != nil {
		return nil, fmt.Errorf("%w failed to perform CreateRule request", err)
	}

	rule.SecopsRuleDTO = *ruledto

	ruleNameShort := strings.ReplaceAll(rule.Name, "\"", "")
	ruleNameShort = strings.TrimPrefix(ruleNameShort, "projects/826325533751/locations/eu/instances/87b64822-b99b-453e-a4bc-ab380e798f87/rules/")

	r.Deployment.Name = rule.Name

	deployment, err := c.updateDeployment(ctx, ruleNameShort, r.Deployment)
	if err != nil {
		return nil, err
	}

	rule.Deployment = *deployment

	return &rule, nil
}
