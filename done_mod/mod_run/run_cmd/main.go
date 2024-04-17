package runcmd

import (
	"log/slog"
	"os"

	runcfg "github.com/hiveot/hub/done_mod/mod_run/run_cfg"
	runsrv "github.com/hiveot/hub/done_mod/mod_run/run_srv"
	"github.com/hiveot/hub/done_tool/logging"
	"github.com/hiveot/hub/done_tool/plugin"
)

// Connect the launcher service
func main() {
	// setup environment and config
	env := plugin.GetAppEnvironment("", true)
	logging.SetLogging(env.LogLevel, "")

	cfg := runcfg.NewLauncherConfig()
	cfg.LogLevel = env.LogLevel
	err := env.LoadConfig(&cfg)
	if err != nil {
		slog.Error("Failed loading launcher config: ", "err", err)
		os.Exit(1)
	}
	logging.SetLogging(cfg.LogLevel, "")

	// start the launcher but do not connect yet as the core can be started by the launcher itself.
	// the core will generate the launcher key and token.
	svc := runsrv.NewLauncherService(env, cfg, nil)
	err = svc.Start()
	if err != nil {
		slog.Error("Failed starting launcher: ", "err", err)
		// we're going to exit. Don't leave the core running
		_ = svc.Stop()
		os.Exit(1)
	}

	// wait for a stop signal
	runsrv.WaitForSignal()
	err = svc.Stop()
	if err != nil {
		os.Exit(2)
	}
}
