package authservice

import (
	"log/slog"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	"github.com/hiveot/hub/done_cli/cli_done/transport"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	modbus "github.com/hiveot/hub/done_mod/mod_bus"
)

// AuthManageRoles manages custom roles.
// Intended for administrators.
//
// This implements the IAuthManageRoles interface.
type AuthManageRoles struct {
	// Client record persistence
	store authapi.IAuthnStore
	// message server for apply role changes
	msgServer modbus.IMsgServer
	// action subscription
	actionSub transport.ISubscription
	// message server connection
	hc *clidone.HubClient
}

// CreateRole adds a new custom role
func (svc *AuthManageRoles) CreateRole(args authapi.CreateRoleArgs) error {
	// FIXME:implement
	slog.Error("CreateRole is not yet implemented")
	return nil
}

// DeleteRole deletes a custom role
func (svc *AuthManageRoles) DeleteRole(args authapi.DeleteRoleArgs) error {
	// FIXME:implement
	slog.Error("DeleteRole is not yet implemented")
	return nil
}

// HandleRequest unmarshal and apply action requests
//func (svc *AuthManageRoles) HandleRequest(msg *things.ThingValue) (reply []byte, err error) {
//
//	slog.Info("HandleRequest",
//		slog.String("actionID", msg.Name),
//		slog.String("senderID", msg.SenderID))
//	switch msg.Name {
//	case authapi.CreateRoleReq:
//		req := &authapi.CreateRoleArgs{}
//		err := ser.Unmarshal(msg.Data, &req)
//		if err != nil {
//			return nil, err
//		}
//		err = svc.CreateRole(req.Role)
//		return nil, err
//	case authapi.DeleteRoleReq:
//		req := &authapi.DeleteRoleArgs{}
//		err := ser.Unmarshal(msg.Data, &req)
//		if err != nil {
//			return nil, err
//		}
//		err = svc.DeleteRole(req.Role)
//		return nil, err
//
//	default:
//		return nil, fmt.Errorf("unknown action '%s' for client '%s'", msg.Name, msg.SenderID)
//	}
//}

// Start subscribes to the actions for management and client capabilities
// Register the binding subscription using the given connection
func (svc *AuthManageRoles) Start() (err error) {
	if svc.hc != nil {
		svc.hc.SetRPCCapability(authapi.AuthManageRolesCapability,
			map[string]interface{}{
				authapi.CreateRoleReq: svc.CreateRole,
				authapi.DeleteRoleReq: svc.DeleteRole,
			})
	}
	return err
}

// Stop removes subscriptions
func (svc *AuthManageRoles) Stop() {
	if svc.actionSub != nil {
		svc.actionSub.Unsubscribe()
		svc.actionSub = nil
	}
}

// NewAuthManageRoles creates the auth role management capability
func NewAuthManageRoles(
	store authapi.IAuthnStore,
	hc *clidone.HubClient,
	msgServer modbus.IMsgServer) *AuthManageRoles {

	svc := AuthManageRoles{
		store:     store,
		hc:        hc,
		msgServer: msgServer,
	}
	return &svc
}
