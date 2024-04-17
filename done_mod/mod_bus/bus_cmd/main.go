package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	donecfg "github.com/hiveot/hub/done_cfg"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	authservice "github.com/hiveot/hub/done_mod/mod_auth/auth_srv"
	bussrv "github.com/hiveot/hub/done_mod/mod_bus/bus_srv"
	"github.com/hiveot/hub/done_tool/discovery"
	"github.com/hiveot/hub/done_tool/logging"
	"github.com/hiveot/hub/done_tool/plugin"
)

// Launch the hub NATS core
//
// This starts the embedded messaging service and in-process core services.
//
// commandline:  natscore [options]
// Run with '-h' to see the application environment options.
//
// This runs HubCoredonecfg.Setup which creates missing directories, certs and
// auth keys and tokens.
func main() {
	flag.Usage = func() {
		fmt.Println("Usage: natscore [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
	}
	env := plugin.GetAppEnvironment("", true)
	//env.Core = "nats"
	logging.SetLogging(env.LogLevel, "")
	fmt.Println("home: ", env.HomeDir)
	if len(flag.Args()) > 0 {
		println("No arguments expected.")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// setup the core configuration
	hubCfg := donecfg.NewHubCoreConfig()
	err := hubCfg.Setup(&env, false)
	if err != nil {
		fmt.Println("ERROR:", err.Error())
		os.Exit(1)
	}
	err = run(hubCfg)
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

// run starts the server and core services
// This does not return until a signal is received
func run(cfg *donecfg.HubCoreConfig) error {
	var err error

	msgServer := bussrv.NewNatsMsgServer(&cfg.NatsServer, authapi.DefaultRolePermissions)
	err = msgServer.Start()

	if err != nil {
		return fmt.Errorf("unable to start server: %w", err)
	}

	// Start the auth service. NATS requires brcypt passwords
	authSvc, _ := authservice.StartAuthService(cfg.Auth, msgServer, cfg.CaCert)

	// start discovery
	serverURL, _, _ := msgServer.GetServerURLs()
	if cfg.EnableMDNS {
		urlInfo, err := url.Parse(serverURL)
		if err != nil {
			return err
		}
		port, _ := strconv.Atoi(urlInfo.Port())
		svc, err := discovery.ServeDiscovery(
			"natscore", "hiveot", urlInfo.Host, port, map[string]string{
				"rawurl": serverURL,
				"core":   "nats",
			})
		_ = svc
		_ = err
	}

	// wait until signal
	fmt.Println("NATS Hub Core started. ClientURL=" + serverURL)
	plugin.WaitForSignal()

	authSvc.Stop()
	msgServer.Stop()
	// give background tasks time to stop
	time.Sleep(time.Millisecond * 100)
	return nil
}
