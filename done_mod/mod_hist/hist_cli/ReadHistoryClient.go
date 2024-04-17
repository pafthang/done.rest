package histcli

import (
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	histapi "github.com/hiveot/hub/done_mod/mod_hist/hist_api"
	"github.com/hiveot/hub/done_tool/things"
)

// ReadHistoryClient for talking to the history service
type ReadHistoryClient struct {
	// service providing the history capability
	serviceID string
	// capability to use
	capID string
	hc    *clidone.HubClient
}

// GetCursor returns an iterator for ThingValue objects containing historical events,tds or actions
// This returns a release function that MUST be called after completion.
//
//	agentID of the publisher of the event or action
//	thingID the event or action belongs to
//	name option filter on a specific event or action name
func (cl *ReadHistoryClient) GetCursor(
	agentID string, thingID string, name string) (cursor *HistoryCursorClient, releaseFn func(), err error) {
	req := histapi.GetCursorArgs{
		AgentID: agentID,
		ThingID: thingID,
		Name:    name,
	}
	resp := histapi.GetCursorResp{}
	err = cl.hc.PubRPCRequest(cl.serviceID, cl.capID, histapi.GetCursorMethod, &req, &resp)
	cursor = NewHistoryCursorClient(cl.hc, cl.serviceID, cl.capID, resp.CursorKey)
	return cursor, cursor.Release, err
}

// GetLatest returns the latest values of a Thing
//
//	agentID of the publisher of the event or action
//	thingID the event or action belongs to
//	names optionally filter on specific property, event or action names. nil for all values
func (cl *ReadHistoryClient) GetLatest(
	agentID string, thingID string, names []string) (things.ThingValueMap, error) {
	args := histapi.GetLatestArgs{
		AgentID: agentID,
		ThingID: thingID,
		Names:   names,
	}
	resp := histapi.GetLatestResp{}
	err := cl.hc.PubRPCRequest(cl.serviceID, cl.capID, histapi.GetLatestMethod, &args, &resp)
	return resp.Values, err
}

// NewReadHistoryClient returns an instance of the read history client using the given connection
func NewReadHistoryClient(hc *clidone.HubClient) *ReadHistoryClient {
	histCl := ReadHistoryClient{
		hc:        hc,
		serviceID: histapi.ServiceName,
		capID:     histapi.ReadHistoryCap,
	}
	return &histCl
}
