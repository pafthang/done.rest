package runcli

import (
	"fmt"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	runapi "github.com/hiveot/hub/done_mod/mod_run/run_api"
)

// LauncherClient is a marshaller for service messages using a provided hub connection.
// This uses the default serializer to marshal and unmarshal messages.
type LauncherClient struct {
	// ID of the launcher service that handles the requests
	agentID string
	capID   string
	hc      *clidone.HubClient
}

// List services
func (cl *LauncherClient) List(onlyRunning bool) ([]runapi.PluginInfo, error) {

	req := runapi.ListArgs{
		OnlyRunning: onlyRunning,
	}
	resp := runapi.ListResp{}
	err := cl.hc.PubRPCRequest(cl.agentID, cl.capID, runapi.ListMethod, req, &resp)
	return resp.PluginInfoList, err
}

// Start cannot start remotely
func (cl *LauncherClient) Start() error {
	return fmt.Errorf("cannot start launcher remotely")
}

// StartPlugin requests to start a plugin
func (cl *LauncherClient) StartPlugin(name string) (runapi.PluginInfo, error) {

	req := runapi.StartPluginArgs{
		Name: name,
	}
	resp := runapi.StartPluginResp{}
	err := cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, runapi.StartPluginMethod, req, &resp)
	return resp.PluginInfo, err
}

// StartAllPlugins starts all enabled plugins
// This returns the error from the last service that could not be started
func (cl *LauncherClient) StartAllPlugins() error {
	err := cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, runapi.StartAllPluginsMethod, nil, nil)
	return err
}

// Stop cannot stop remotely
func (cl *LauncherClient) Stop() error {
	return fmt.Errorf("cannot stop launcher remotely")
}

// StopPlugin stops a running plugin
func (cl *LauncherClient) StopPlugin(name string) (runapi.PluginInfo, error) {
	req := runapi.StopPluginArgs{
		Name: name,
	}
	resp := runapi.StopPluginResp{}
	err := cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, runapi.StopPluginMethod, req, &resp)
	return resp.PluginInfo, err
}

// StopAllPlugins stops running plugins
func (cl *LauncherClient) StopAllPlugins() error {
	req := runapi.StopAllPluginsArgs{
		IncludingCore: false,
	}
	err := cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, runapi.StopAllPluginsMethod, &req, nil)
	return err
}

// NewLauncherClient returns a launcher service client
//
//	launcherID is the optional ID of the launcher to use. Default is 'launcher'
//	hc is the hub client connection to use.
func NewLauncherClient(launcherID string, hc *clidone.HubClient) *LauncherClient {
	if launcherID == "" {
		launcherID = runapi.ServiceName
	}
	cl := LauncherClient{
		hc:      hc,
		agentID: launcherID,
		capID:   runapi.ManageCapability,
	}
	return &cl
}
