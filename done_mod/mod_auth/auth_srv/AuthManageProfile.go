package authservice

import (
	"crypto/x509"
	"fmt"
	"log/slog"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	modbus "github.com/hiveot/hub/done_mod/mod_bus"
)

// AuthManageProfile is the capability for clients to view and update their own profile.
type AuthManageProfile struct {
	// Client record persistence
	store authapi.IAuthnStore
	// hub client for subscribing to requests
	hc *clidone.HubClient

	// message server for updating authn
	msgServer modbus.IMsgServer
	// CA certificate for validating cert
	caCert *x509.Certificate
}

// GetProfile returns a client's profile
func (svc *AuthManageProfile) GetProfile(ctx clidone.ServiceContext) (
	resp *authapi.GetProfileResp, err error) {
	clientProfile, err := svc.store.GetProfile(ctx.SenderID)
	resp = &authapi.GetProfileResp{Profile: clientProfile}
	return resp, err
}

// NewToken validates a password and issues an authn token. A public key must be on file.
func (svc *AuthManageProfile) NewToken(
	ctx clidone.ServiceContext, args authapi.NewTokenArgs) (resp *authapi.NewTokenResp, err error) {

	clientProfile, err := svc.store.VerifyPassword(ctx.SenderID, args.Password)
	if err != nil {
		return resp, err
	}
	authInfo := modbus.ClientAuthInfo{
		ClientID:   clientProfile.ClientID,
		ClientType: clientProfile.ClientType,
		PubKey:     clientProfile.PubKey,
		Role:       clientProfile.Role,
	}
	newToken, err := svc.msgServer.CreateToken(authInfo)
	resp = &authapi.NewTokenResp{Token: newToken}
	return resp, err
}

// notification handler invoked when clients have been updated
// this invokes a reload of server authn
func (svc *AuthManageProfile) onChange() {
	// wait with applying credential changes to allow a response to be send
	go svc.msgServer.ApplyAuth(svc.store.GetAuthClientList())
}

// RefreshToken issues a new token for the authenticated user.
// This returns a refreshed token that can be used to connect to the messaging server
// the old token must be a valid jwt token belonging to the clientID
func (svc *AuthManageProfile) RefreshToken(ctx clidone.ServiceContext) (*authapi.RefreshTokenResp, error) {
	// verify the token
	clientProfile, err := svc.store.GetProfile(ctx.SenderID)
	if err != nil {
		return nil, err
	}
	//err = svc.msgServer.ValidateToken(
	//	clientID, clientProfile.PubKey, oldToken, "", "")
	//if err != nil {
	//	return "", fmt.Errorf("error validating oldToken of client %s: %w", clientID, err)
	//}
	authInfo := modbus.ClientAuthInfo{
		ClientID:   clientProfile.ClientID,
		ClientType: clientProfile.ClientType,
		PubKey:     clientProfile.PubKey,
		Role:       clientProfile.Role,
	}
	newToken, err := svc.msgServer.CreateToken(authInfo)
	if err != nil {
		slog.Warn("RefreshToken",
			"clientID", clientProfile.ClientID, "err", err.Error())
	}
	resp := &authapi.RefreshTokenResp{Token: newToken}
	return resp, err
}

// SetServicePermissions sets the client roles that are allowed to use this service.
// Intended for use by services to set the roles that have access to it.
// This fails if this client is not a service.
func (svc *AuthManageProfile) SetServicePermissions(
	ctx clidone.ServiceContext, args *authapi.SetServicePermissionsArgs) error {
	// the client must be a service

	clientProfile, err := svc.store.GetProfile(ctx.SenderID)
	if err != nil {
		return err
	} else if clientProfile.ClientType != authapi.ClientTypeService {
		return fmt.Errorf("client '%s' must be a service, not a '%s'", ctx.SenderID, clientProfile.ClientType)
	}

	svc.msgServer.SetServicePermissions(ctx.SenderID, args.Capability, args.Roles)
	return nil
}

// Start subscribes to the actions for management and client capabilities
// Register the binding subscription using the given connection
func (svc *AuthManageProfile) Start() (err error) {
	if svc.hc != nil {

		svc.hc.SetRPCCapability(
			authapi.AuthProfileCapability, map[string]interface{}{
				authapi.GetProfileMethod:            svc.GetProfile,
				authapi.NewTokenMethod:              svc.NewToken,
				authapi.RefreshTokenMethod:          svc.RefreshToken,
				authapi.SetServicePermissionsMethod: svc.SetServicePermissions,
				authapi.UpdateNameMethod:            svc.UpdateName,
				authapi.UpdatePasswordMethod:        svc.UpdatePassword,
				authapi.UpdatePubKeyMethod:          svc.UpdatePubKey,
			})
	}
	return err
}

// Stop removes subscriptions
func (svc *AuthManageProfile) Stop() {
	//if svc.actionSub != nil {
	//	svc.actionSub.Unsubscribe()
	//	svc.actionSub = nil
	//}
}

func (svc *AuthManageProfile) UpdateName(
	ctx clidone.ServiceContext, args *authapi.UpdateNameArgs) (err error) {

	clientProfile, _ := svc.store.GetProfile(ctx.SenderID)
	clientProfile.DisplayName = args.NewName
	err = svc.store.Update(ctx.SenderID, clientProfile)
	// this doesn't affect authentication
	return err
}

func (svc *AuthManageProfile) UpdatePassword(
	ctx clidone.ServiceContext, args *authapi.UpdatePasswordArgs) (err error) {
	slog.Info("UpdatePassword", "clientID", ctx.SenderID)
	_, err = svc.GetProfile(ctx)
	if err != nil {
		return err
	}
	err = svc.store.SetPassword(ctx.SenderID, args.NewPassword)
	if err != nil {
		return err
	}
	svc.onChange()
	return err
}

func (svc *AuthManageProfile) UpdatePubKey(
	ctx clidone.ServiceContext, args *authapi.UpdatePubKeyArgs) (err error) {

	slog.Info("UpdatePubKey", "clientID", ctx.SenderID)
	clientProfile, err := svc.store.GetProfile(ctx.SenderID)
	if err != nil {
		return err
	}
	clientProfile.PubKey = args.NewPubKey
	err = svc.store.Update(ctx.SenderID, clientProfile)
	if err != nil {
		return err
	}
	// run in the background so a response can be sent
	go svc.onChange()
	return err
}

// NewAuthManageProfile returns a user profile management capability.
//
//	store holds the authentication client records
//	caCert is an optional CA used to verify certificates. Use nil to not authn using client certs
func NewAuthManageProfile(
	store authapi.IAuthnStore,
	caCert *x509.Certificate,
	hc *clidone.HubClient,
	msgServer modbus.IMsgServer,
) *AuthManageProfile {

	svc := &AuthManageProfile{
		store:     store,
		hc:        hc,
		msgServer: msgServer,
		caCert:    caCert,
	}
	return svc
}
