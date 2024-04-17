package main

import (
	"fmt"
	"log/slog"
	"os"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	doneauth "github.com/hiveot/hub/done_cmd/cmd_done/done_auth"
	donecert "github.com/hiveot/hub/done_cmd/cmd_done/done_cert"
	donedir "github.com/hiveot/hub/done_cmd/cmd_done/done_dir"
	donehist "github.com/hiveot/hub/done_cmd/cmd_done/done_hist"
	doneprov "github.com/hiveot/hub/done_cmd/cmd_done/done_prov"
	donepubsub "github.com/hiveot/hub/done_cmd/cmd_done/done_pubsub"
	donerun "github.com/hiveot/hub/done_cmd/cmd_done/done_run"
	donesetup "github.com/hiveot/hub/done_cmd/cmd_done/done_setup"
	"github.com/hiveot/hub/done_tool/logging"
	"github.com/hiveot/hub/done_tool/plugin"
	"github.com/hiveot/hub/done_tool/utils"
	"github.com/urfave/cli/v2"
)

const Version = `0.1-alpha`

// var env utils.AppEnvironment
var nowrap bool

// CLI for managing the HiveOT Hub
//
// commandline:  hubcli command options

func main() {
	var hc *clidone.HubClient
	var verbose bool
	var loginID = "admin"
	var password = ""
	var homeDir string
	var certsDir string
	var serverURL string

	// environment defaults
	env := plugin.GetAppEnvironment("", false)
	homeDir = env.HomeDir
	certsDir = env.CertsDir

	//defaultHome := env.HomeDir // to detect changes to the home directory
	logging.SetLogging("warning", "")
	nowrap = false

	app := &cli.App{
		EnableBashCompletion: true,
		Name:                 "hubcli",
		Usage:                "Hub Commandline Interface",
		Version:              Version,

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "home",
				Usage:       "Path to application home directory",
				Value:       homeDir,
				Destination: &homeDir,
			},
			&cli.BoolFlag{
				Name:        "nowrap",
				Usage:       "Disable konsole wrapping",
				Value:       nowrap,
				Destination: &nowrap,
			},
			&cli.StringFlag{
				Name:        "login",
				Usage:       "login ID",
				Value:       loginID,
				Destination: &loginID,
			},
			&cli.StringFlag{
				Name:        "password",
				Usage:       "optional password for alt user",
				Value:       password,
				Destination: &password,
			},
			&cli.StringFlag{
				Name:        "server",
				Usage:       "server URL (default: use DNS-SD discovery)",
				Value:       serverURL,
				Destination: &serverURL,
			},
			&cli.BoolFlag{
				Name:        "loginfo",
				Usage:       "verbose logging",
				Value:       verbose,
				Destination: &verbose,
			},
		},
		Before: func(c *cli.Context) (err error) {
			// reload env in case home changes
			env = plugin.GetAppEnvironment(homeDir, false)
			certsDir = env.CertsDir
			if verbose {
				logging.SetLogging("info", "")
			}
			if nowrap {
				fmt.Printf(utils.WrapOff)
			}
			// todo: don't connect when running setup
			hc, err = clidone.ConnectToHub(serverURL, loginID, certsDir, "", password)
			if err != nil {
				slog.Warn("Unable to connect to the server", "err", err)
			}
			return nil
		},
		// commands arguments are passed by reference so they are updated in the Before section
		Commands: []*cli.Command{
			// these commands work without a server connection
			donecert.CreateCACommand(&certsDir),
			donecert.ViewCACommand(&certsDir),
			donesetup.SetupCommand(&env),

			doneauth.AuthAddUserCommand(&hc),
			doneauth.AuthAddServiceCommand(&hc, &env.CertsDir),
			doneauth.AuthListClientsCommand(&hc),
			doneauth.AuthRemoveClientCommand(&hc),
			doneauth.AuthSetPasswordCommand(&hc),

			donerun.LauncherListCommand(&hc),
			donerun.LauncherStartCommand(&hc),
			donerun.LauncherStopCommand(&hc),

			donedir.DirectoryListCommand(&hc),

			donehist.HistoryLatestCommand(&hc),
			donehist.HistoryListCommand(&hc),

			donepubsub.PubActionCommand(&hc),
			donepubsub.SubEventsCommand(&hc),
			donepubsub.SubTDCommand(&hc),

			doneprov.ProvisionListCommand(&hc),
			doneprov.ProvisionRequestCommand(&hc),
			doneprov.ProvisionApproveRequestCommand(&hc),
			doneprov.ProvisionPreApproveCommand(&hc),
		},
	}

	// Show the arguments in the command line
	//	cli.AppHelpTemplate = `NAME:
	//  {{.ID}} - {{.Usage}}
	//USAGE:
	//  {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
	//  {{if len .Authors}}
	//AUTHOR:
	//  {{range .Authors}}{{ . }}{{end}}
	//  {{end}}{{if .Commands}}
	//COMMANDS: {{range .VisibleCategories}}{{if .ID}}
	//   {{.ID }}:{{"\t"}}{{range .VisibleCommands}}
	//      {{join .Names ", "}} {{.ArgsUsage}} {{"\t"}}{{.Usage}}{{end}}{{else}}{{template "visibleCommandTemplate" .}}{{end}}{{end}}
	//
	//GLOBAL OPTIONS:
	//  {{range .VisibleFlags}}{{.}}
	//  {{end}}
	//{{end}}
	//`
	app.Suggest = true
	app.HideHelpCommand = true
	if err := app.Run(os.Args); err != nil {
		println("ERROR: ", err.Error())
		//helpArgs := append(os.Args, "-h")
		//_ = app.Run(helpArgs)
	}
}
