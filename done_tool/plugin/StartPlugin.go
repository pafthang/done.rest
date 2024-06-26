package plugin

import (
	"log/slog"
	"os"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	"github.com/hiveot/hub/done_tool/logging"
)

type PluginConfig struct {
	LogLevel string `yaml:""`
}

// IPlugin interface of protocol bindings and service plugins
type IPlugin interface {
	// Start the plugin with the given environment settings and hub connection
	//	hc is the hub connection for publishing and subscribing
	Start(hc *clidone.HubClient) error
	Stop()
}

// StartPlugin implements the boilerplate to launch a plugin based on argv
// and its config. This does not return until a signal is received.
//
// The plugin clientID is the binary name obtained from argv[0]. It can be
// obtained from hc.ClientID()
//
//	plugin is the instance of the plugin
func StartPlugin(plugin IPlugin, env *AppEnvironment) {

	// setup environment and config
	//env := GetAppEnvironment("", true)
	logging.SetLogging(env.LogLevel, "")

	// locate the hub, load CA certificate, load service key and token and connect
	hc, err := clidone.ConnectToHub("", env.ClientID, env.CertsDir, "", "")
	if err != nil {
		slog.Error("Failed connecting to the Hub", "err", err)
		os.Exit(1)
	}
	// start the service
	err = plugin.Start(hc)
	if err != nil {
		slog.Error("failed starting service", "err", err.Error())
		os.Exit(1)
	}
	WaitForSignal()
	plugin.Stop()

	os.Exit(0)

}
