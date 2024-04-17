// Package main with the history store
package main

import (
	"fmt"
	"log/slog"
	"path"

	histcfg "github.com/hiveot/hub/done_mod/mod_hist/hist_cfg"
	histsrv "github.com/hiveot/hub/done_mod/mod_hist/hist_srv"
	"github.com/hiveot/hub/done_tool/buckets/boltstore"
	"github.com/hiveot/hub/done_tool/logging"
	"github.com/hiveot/hub/done_tool/plugin"
)

// Connect the history store service
func main() {
	env := plugin.GetAppEnvironment("", true)
	logging.SetLogging(env.LogLevel, "")
	slog.Warn("Starting history service", "clientID", env.ClientID, "loglevel", env.LogLevel)

	storesDir := path.Join(env.StoresDir, env.ClientID)
	cfg := histcfg.NewHistoryConfig(storesDir)
	_ = env.LoadConfig(&cfg)

	// the service uses the bucket store to store history
	store := boltstore.NewBoltsStore(cfg.StoreDirectory, "history", cfg.Backend)
	err := store.Open()
	if err != nil {
		err = fmt.Errorf("can't open history bucket store: %w", err)
		slog.Error(err.Error())
		panic(err.Error())
	}
	svc := histsrv.NewHistoryService(store)
	plugin.StartPlugin(svc, &env)
}
