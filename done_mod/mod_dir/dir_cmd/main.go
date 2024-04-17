// Package main with the things directory store
package main

import (
	"log/slog"
	"path"

	dirsrv "github.com/hiveot/hub/done_mod/mod_dir/dir_srv"
	"github.com/hiveot/hub/done_tool/buckets/bolts"
	"github.com/hiveot/hub/done_tool/logging"
	"github.com/hiveot/hub/done_tool/plugin"
)

// name of the storage file
const storeFile = "directory.kvbtree"

// Start the service.
// Precondition: A loginID and keys for this service must already have been added.
// This can be done manually using the hubcli or simply be starting it using the launcher.
func main() {
	env := plugin.GetAppEnvironment("", true)
	logging.SetLogging(env.LogLevel, "")
	slog.Warn("Starting directory service", "clientID", env.ClientID, "loglevel", env.LogLevel)

	// startup
	storePath := path.Join(env.StoresDir, env.ClientID, storeFile)
	store := bolts.NewBoltStore(storePath)
	err := store.Open()
	if err != nil {
		panic("unable to open the directory store")
	}
	svc := dirsrv.NewDirectoryService(store)
	plugin.StartPlugin(svc, &env)
}
