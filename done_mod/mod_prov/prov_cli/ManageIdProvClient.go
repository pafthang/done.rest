package provcli

import (
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	provapi "github.com/hiveot/hub/done_mod/mod_prov/prov_api"
)

// ManageIdProvClient is a hiveot client for communicating with the provisioning
// service using the message bus.
// This requires admin permissions.
type ManageIdProvClient struct {
	hc *clidone.HubClient
	// agentID of the service
	serviceID string
	// capabilityID of this capability
	capID string
}

// ApproveRequest approves a pending provisioning request
func (cl *ManageIdProvClient) ApproveRequest(ClientID string, clientType string) error {
	args := provapi.ApproveRequestArgs{
		ClientID:   ClientID,
		ClientType: clientType,
	}
	err := cl.hc.PubRPCRequest(cl.serviceID, cl.capID,
		provapi.ApproveRequestMethod, &args, nil)

	return err
}

// GetRequests returns requests
// Expired requests are not included.
func (cl *ManageIdProvClient) GetRequests(
	pending, approved, rejected bool) ([]provapi.ProvisionStatus, error) {
	args := provapi.GetRequestsArgs{
		Pending:  pending,
		Approved: approved,
		Rejected: rejected,
	}
	resp := provapi.GetRequestsResp{}
	err := cl.hc.PubRPCRequest(cl.serviceID, cl.capID,
		provapi.GetRequestsMethod, &args, &resp)

	return resp.Requests, err
}

// PreApproveDevices uploads a list of pre-approved devices ID, MAC and PubKey
func (cl *ManageIdProvClient) PreApproveDevices(
	approvals []provapi.PreApprovedClient) error {

	args := provapi.PreApproveClientsArgs{
		Approvals: approvals,
	}
	err := cl.hc.PubRPCRequest(cl.serviceID, cl.capID,
		provapi.PreApproveClientsMethod, &args, nil)

	return err
}

// RejectRequest rejects a pending provisioning request
func (cl *ManageIdProvClient) RejectRequest(clientID string) error {
	args := provapi.RejectRequestArgs{ClientID: clientID}
	err := cl.hc.PubRPCRequest(cl.serviceID, cl.capID,
		provapi.RejectRequestMethod, &args, nil)

	return err
}
func (cl *ManageIdProvClient) SubmitRequest(
	clientID string, pubKey string, mac string) (
	status *provapi.ProvisionStatus, token string, err error) {

	args := provapi.SubmitRequestArgs{
		ClientID:   clientID,
		ClientType: authapi.ClientTypeDevice,
		PubKey:     pubKey,
		MAC:        mac,
	}
	resp := provapi.ProvisionRequestResp{}
	err = cl.hc.PubRPCRequest(cl.serviceID, cl.capID,
		provapi.SubmitRequestMethod, &args, &resp)

	return &resp.Status, resp.Token, err
}

func NewIdProvManageClient(hc *clidone.HubClient) *ManageIdProvClient {
	cl := &ManageIdProvClient{
		hc: hc,
		//
		serviceID: provapi.ServiceName,
		capID:     provapi.ManageProvisioningCap,
	}
	return cl
}
