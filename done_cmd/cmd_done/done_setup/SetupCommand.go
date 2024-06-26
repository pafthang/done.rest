package donesetup

import (
	"fmt"

	donecfg "github.com/hiveot/hub/done_cfg"
	"github.com/hiveot/hub/done_tool/plugin"
	"github.com/urfave/cli/v2"
)

// SetupCommand creates the environment setup
func SetupCommand(env *plugin.AppEnvironment) *cli.Command {
	newSetup := false
	core := "mqtt"
	return &cli.Command{
		Name:      "setup",
		Usage:     "Create missing directories and credentials for core",
		ArgsUsage: "mqtt | nats",
		//Category: "launcher",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "new",
				Usage:       "Delete the existing setup. Use with care!",
				Value:       newSetup,
				Destination: &newSetup,
			},
		},
		Action: func(cCtx *cli.Context) error {
			core = cCtx.Args().First()
			if core != "mqtt" && core != "nats" {
				return fmt.Errorf("expected core mqtt or nats")
			}
			err := HandleSetup(env, core, newSetup)
			return err
		},
	}
}

// HandleSetup ensure the hiveot environment is setup properly
func HandleSetup(env *plugin.AppEnvironment, core string, newSetup bool) error {
	var err error
	coreConfig := donecfg.NewHubCoreConfig()
	err = coreConfig.Setup(env, newSetup)
	if err != nil {
		return err
	}
	//err = coreConfig.Save()
	//if err != nil {
	//	slog.Error("Saving config failed", "err", err)
	//} else {
	//	println("config saved to: ", path.Join(coreConfig.Env.ConfigDir, config.HubCoreConfigFileName))
	//}

	// TODO: generate a default launcher config if it doesn't exist

	// TODO: run this command without attempting to connect
	fmt.Println("Setup for " + core + " completed successfully. (ignore any connection errors)")
	fmt.Println("(next, run bin/launcher)")
	return err
}
