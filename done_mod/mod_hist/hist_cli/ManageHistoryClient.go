package histcli

import (
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	histapi "github.com/hiveot/hub/done_mod/mod_hist/hist_api"
)

// ManageHistoryClient client for managing retention of the history service
type ManageHistoryClient struct {
	// service providing the history capability
	serviceID string
	// capability to use
	capID string
	hc    *clidone.HubClient
}

// GetRetentionRule returns the retention configuration of an event by name
// This applies to events from any publishers and things
// returns nil if there is no retention rule for the event
//
//	eventName whose retention to return
func (cl *ManageHistoryClient) GetRetentionRule(agentID string, thingID string, name string) (*histapi.RetentionRule, error) {
	args := histapi.GetRetentionRuleArgs{
		AgentID: agentID,
		ThingID: thingID,
		Name:    name,
	}
	resp := histapi.GetRetentionRuleResp{}
	err := cl.hc.PubRPCRequest(
		cl.serviceID, cl.capID, histapi.GetRetentionRuleMethod, &args, &resp)
	return resp.Rule, err
}

// GetRetentionRules returns the list of retention rules
func (cl *ManageHistoryClient) GetRetentionRules() (histapi.RetentionRuleSet, error) {
	resp := histapi.GetRetentionRulesResp{}
	err := cl.hc.PubRPCRequest(cl.serviceID, cl.capID, histapi.GetRetentionRulesMethod, nil, &resp)
	return resp.Rules, err
}

// SetRetentionRules configures the retention of a Thing event
func (cl *ManageHistoryClient) SetRetentionRules(rules histapi.RetentionRuleSet) error {
	args := histapi.SetRetentionRulesArgs{Rules: rules}
	err := cl.hc.PubRPCRequest(
		cl.serviceID, cl.capID, histapi.SetRetentionRulesMethod, &args, nil)
	return err
}

// NewManageHistoryClient creates a new instance of the manage history client for use by authorized clients
func NewManageHistoryClient(hc *clidone.HubClient) *ManageHistoryClient {
	mngCl := &ManageHistoryClient{
		serviceID: histapi.ServiceName,
		capID:     histapi.ManageHistoryCap,
		hc:        hc,
	}
	return mngCl
}
