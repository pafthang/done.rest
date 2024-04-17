package authcli

import (
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
)

// RolesClient is a marshaller for messaging to manage custom roles
// This uses the default serializer to marshal and unmarshal messages.
type RolesClient struct {
	// ID of the authn service agent
	agentID string
	// capability to invoke
	capID string
	hc    *clidone.HubClient
}

// CreateRole creates a new custom role
func (cl *RolesClient) CreateRole(role string) error {

	req := authapi.CreateRoleArgs{
		Role: role,
	}
	err := cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, authapi.CreateRoleReq, &req, nil)
	return err
}

// DeleteRole deletes a custom role
func (cl *RolesClient) DeleteRole(role string) error {

	req := authapi.DeleteRoleArgs{
		Role: role,
	}
	err := cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, authapi.DeleteRoleReq, &req, nil)
	return err
}

// NewRolesClient creates a new client for managing roles
//
//	hc is the hub client connection to use
func NewRolesClient(hc *clidone.HubClient) *RolesClient {
	cl := &RolesClient{
		agentID: authapi.AuthServiceName,
		capID:   authapi.AuthManageRolesCapability,
		hc:      hc,
	}
	return cl

}
