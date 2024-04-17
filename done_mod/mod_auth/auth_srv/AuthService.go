package authservice

import (
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	authcfg "github.com/hiveot/hub/done_mod/mod_auth/auth_cfg"
	authstr "github.com/hiveot/hub/done_mod/mod_auth/auth_str"
	modbus "github.com/hiveot/hub/done_mod/mod_bus"
)

// AuthService handles authentication and authorization requests
type AuthService struct {
	store     authapi.IAuthnStore
	msgServer modbus.IMsgServer
	caCert    *x509.Certificate

	// the hub client connection to listen to requests
	cfg        authcfg.AuthConfig
	hc         *clidone.HubClient
	MngClients *AuthManageClients
	MngRoles   *AuthManageRoles
	MngProfile *AuthManageProfile
}

// Start the service and activate the binding to handle requests
// This adds an 'auth' and 'launcher' client with key in the admin.key/launcher.key files.
func (svc *AuthService) Start() (err error) {
	clientID := authapi.AuthServiceName
	slog.Warn("starting AuthService", "clientID", clientID)
	err = svc.store.Open()
	if err != nil {
		return err
	}

	// before being able to connect, the AuthService and its key must be known
	tcpAddr, _, udsAddr := svc.msgServer.GetServerURLs()
	svc.hc = clidone.NewHubClient(tcpAddr, clientID, svc.caCert)
	// auth service key are in-memory only
	myKP := svc.hc.CreateKeyPair()
	myPubKey := myKP.ExportPublic()

	// use a temporary instance of the client manager to add itself
	mngClients := NewAuthManageClients(svc.store, nil, svc.msgServer)
	args1 := authapi.AddServiceArgs{
		ServiceID:   clientID,
		DisplayName: "Auth Service",
		PubKey:      myPubKey,
	}
	ctx := clidone.ServiceContext{SenderID: clientID}
	resp1, err := mngClients.AddService(ctx, args1)
	if err != nil {
		return fmt.Errorf("failed to setup the auth service: %w", err)
	}

	// nats doesnt support uds?
	_ = udsAddr

	err = svc.hc.ConnectWithToken(myKP, resp1.Token)

	if err != nil {
		return err
	}
	svc.MngClients = NewAuthManageClients(svc.store, svc.hc, svc.msgServer)
	svc.MngRoles = NewAuthManageRoles(svc.store, svc.hc, svc.msgServer)
	svc.MngProfile = NewAuthManageProfile(svc.store, nil, svc.hc, svc.msgServer)

	err = svc.MngClients.Start()
	if err == nil {
		err = svc.MngRoles.Start()
	}
	if err == nil {
		err = svc.MngProfile.Start()
	}
	if err != nil {
		svc.MngClients.Stop()
		svc.MngRoles.Stop()
		svc.MngProfile.Stop()
		svc.hc.Disconnect()
		return
	}

	// set the client roles required to use the service capabilities
	svc.msgServer.SetServicePermissions(clientID, authapi.AuthManageClientsCapability,
		[]string{authapi.ClientRoleAdmin})
	svc.msgServer.SetServicePermissions(clientID, authapi.AuthManageRolesCapability,
		[]string{authapi.ClientRoleAdmin})
	svc.msgServer.SetServicePermissions(clientID, authapi.AuthProfileCapability,
		[]string{authapi.ClientRoleViewer, authapi.ClientRoleOperator, authapi.ClientRoleManager, authapi.ClientRoleAdmin})

	// FIXME, what are the permissions for other services like certs, launcher, ...?

	// Ensure the launcher client exists and has a saved key and auth token
	launcherID := svc.cfg.LauncherAccountID
	slog.Info("Start (auth). Adding launcher service", "ID", launcherID)
	launcherKey, _ := svc.hc.LoadCreateKeyPair(launcherID, svc.cfg.KeysDir)
	args2 := authapi.AddServiceArgs{
		ServiceID:   launcherID,
		DisplayName: "Launcher Service",
		PubKey:      launcherKey.ExportPublic(),
	}
	resp2, err := svc.MngClients.AddService(ctx, args2)
	if err == nil {
		// remove the readonly token file if it exists, to be able to overwrite
		tokenFile := path.Join(svc.cfg.KeysDir, launcherID+clidone.TokenFileExt)
		_ = os.Remove(tokenFile)
		_ = os.WriteFile(tokenFile, []byte(resp2.Token), 0400)
	}

	// ensure the admin user exists and has a saved key and auth token
	adminID := svc.cfg.AdminAccountID
	slog.Info("Start (auth). Adding admin user", "ID", adminID)
	adminKey, _ := svc.hc.LoadCreateKeyPair(adminID, svc.cfg.KeysDir)
	args3 := authapi.AddUserArgs{
		UserID:      adminID,
		DisplayName: "Administrator",
		PubKey:      adminKey.ExportPublic(),
		Role:        authapi.ClientRoleAdmin,
	}
	resp3, err := svc.MngClients.AddUser(ctx, args3)
	if err == nil {
		// remove the readonly token file if it exists, to be able to overwrite
		tokenFile := path.Join(svc.cfg.KeysDir, adminID+clidone.TokenFileExt)
		_ = os.Remove(tokenFile)
		err = os.WriteFile(tokenFile, []byte(resp3.Token), 0400)
	}
	return err
}

// Stop the service, unsubscribe and disconnect from the server
func (svc *AuthService) Stop() {
	slog.Warn("Stopping AuthService")
	if svc.MngClients != nil {
		svc.MngClients.Stop()
		svc.MngClients = nil
	}
	if svc.MngProfile != nil {
		svc.MngProfile.Stop()
	}
	if svc.MngRoles != nil {
		svc.MngRoles.Stop()
	}
	if svc.hc != nil {
		svc.hc.Disconnect()
	}
	svc.store.Close()
}

// NewAuthService creates an authentication service instance
//
//	store is the client store to store authentication clients
//	msgServer used to apply changes to users, devices and services
func NewAuthService(authConfig authcfg.AuthConfig,
	store authapi.IAuthnStore, msgServer modbus.IMsgServer, caCert *x509.Certificate) *AuthService {

	authnSvc := &AuthService{
		caCert:    caCert,
		cfg:       authConfig,
		store:     store,
		msgServer: msgServer,
	}
	return authnSvc
}

// StartAuthService creates and launch the auth service with the given config
// This creates a password store using the config file and password encryption method.
func StartAuthService(cfg authcfg.AuthConfig, msgServer modbus.IMsgServer, caCert *x509.Certificate) (*AuthService, error) {

	// nats requires bcrypt passwords
	authStore := authstr.NewAuthnFileStore(cfg.PasswordFile)
	authnSvc := NewAuthService(cfg, authStore, msgServer, caCert)
	err := authnSvc.Start()
	if err != nil {
		panic("Cant start Auth service: " + err.Error())
	}
	return authnSvc, err
}
