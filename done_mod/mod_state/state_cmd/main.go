package main

import (
	"log/slog"
	"path"

	statesrv "github.com/hiveot/hub/done_mod/mod_state/state_srv"
	"github.com/hiveot/hub/done_tool/logging"
	"github.com/hiveot/hub/done_tool/plugin"
)

// Start the service.
// Precondition: A loginID and keys for this service must already have been added.
// This can be done manually using the hubcli or simply be starting it using the launcher.
func main() {
	env := plugin.GetAppEnvironment("", true)
	logging.SetLogging(env.LogLevel, "")
	slog.Warn("Starting state service", "clientID", env.ClientID, "loglevel", env.LogLevel)

	// startup
	storePath := path.Join(env.StoresDir, env.ClientID)
	svc := statesrv.NewStateService(storePath)
	plugin.StartPlugin(svc, &env)
}
