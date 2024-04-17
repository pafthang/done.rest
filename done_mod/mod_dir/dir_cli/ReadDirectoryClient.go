// Package capnpclient that wraps the capnp generated client with a POGS API
package dircli

import (
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	dirapi "github.com/hiveot/hub/done_mod/mod_dir/dir_api"
	"github.com/hiveot/hub/done_tool/things"
)

// ReadDirectoryClient is the messenger client for reading the Thing Directory
// This implements the IReadDirectory interface
type ReadDirectoryClient struct {
	// agent handling the request
	agentID string
	// capability to use
	capID string
	hc    *clidone.HubClient
}

// GetCursor returns an iterator for ThingValue objects containing TD documents
func (cl *ReadDirectoryClient) GetCursor() (dirapi.IDirectoryCursor, error) {
	resp := dirapi.GetCursorResp{}
	err := cl.hc.PubRPCRequest(cl.agentID, cl.capID, dirapi.GetCursorMethod, nil, &resp)
	cursor := NewDirectoryCursorClient(cl.hc, cl.agentID, cl.capID, resp.CursorKey)
	return cursor, err
}

// GetTD returns a things value containing the TD document for the given Thing address
// This returns an error if not found
func (cl *ReadDirectoryClient) GetTD(
	agentID string, thingID string) (tv things.ThingValue, err error) {

	req := &dirapi.GetTDArgs{
		AgentID: agentID,
		ThingID: thingID,
	}
	resp := &dirapi.GetTDResp{}
	err = cl.hc.PubRPCRequest(cl.agentID, cl.capID, dirapi.GetTDMethod, &req, &resp)
	return resp.Value, err
}

// GetTDs returns a batch of TD documents.
// The order is undefined.
func (cl *ReadDirectoryClient) GetTDs(
	offset int, limit int) (tv []things.ThingValue, err error) {

	req := &dirapi.GetTDsArgs{
		Offset: offset,
		Limit:  limit,
	}
	resp := &dirapi.GetTDsResp{}
	err = cl.hc.PubRPCRequest(cl.agentID, cl.capID, dirapi.GetTDsMethod, &req, &resp)
	return resp.Values, err
}

// NewReadDirectoryClient creates a instance of a read-directory client
// This connects to the service with the default directory service name.
func NewReadDirectoryClient(hc *clidone.HubClient) *ReadDirectoryClient {
	return &ReadDirectoryClient{
		agentID: dirapi.ServiceName,
		capID:   dirapi.ReadDirectoryCap,
		hc:      hc,
	}
}
