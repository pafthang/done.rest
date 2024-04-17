package runsrv

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	authcli "github.com/hiveot/hub/done_mod/mod_auth/auth_cli"
	runapi "github.com/hiveot/hub/done_mod/mod_run/run_api"
	runcfg "github.com/hiveot/hub/done_mod/mod_run/run_cfg"
	"github.com/hiveot/hub/done_tool/plugin"
)

// LauncherService manages starting and stopping of plugins
// This implements the ILauncher interface
type LauncherService struct {
	// service configuration
	cfg runcfg.LauncherConfig
	env plugin.AppEnvironment

	// map of plugin name to running status
	plugins map[string]*runapi.PluginInfo
	// list of started commands in startup order
	cmds []*exec.Cmd

	// hub messaging client
	hc *clidone.HubClient
	// auth service to generate plugin keys and tokens
	mngAuth *authcli.ManageClients

	// mutex to keep things safe
	mux sync.Mutex
	// watch plugin folders for updates
	serviceWatcher *fsnotify.Watcher
	// service is running
	isRunning atomic.Bool
	// closing channel
	done chan bool
}

// Add discovered core to svc.plugins
func (svc *LauncherService) addCore(coreBin string) error {
	if coreBin != "" {
		corePath := path.Join(svc.env.BinDir, coreBin)
		coreInfo, err := os.Stat(corePath)
		if err != nil {
			err = fmt.Errorf("findCore. core in config not found. Path=%s", corePath)
			return err
		}
		pluginInfo, found := svc.plugins[coreBin]
		if found {
			// update existing entry for core
			pluginInfo.ModifiedTime = coreInfo.ModTime().Format(time.RFC3339)
			pluginInfo.Size = coreInfo.Size()
		} else {
			// add new entry for core
			pluginInfo = &runapi.PluginInfo{
				Name:    coreInfo.Name(),
				Path:    corePath,
				Uptime:  0,
				Running: false,
			}
			pluginInfo.ModifiedTime = coreInfo.ModTime().Format(time.RFC3339)
			pluginInfo.Size = coreInfo.Size()
			svc.plugins[coreBin] = pluginInfo
		}
	}
	return nil
}

// Add newly discovered executable plugins to svc.plugins
// If the service is already know, only update its size and timestamp
func (svc *LauncherService) addPlugins(folder string) error {
	count := 0
	entries, err := os.ReadDir(folder)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		// ignore directories and non executable files
		fileInfo, err := entry.Info()
		if err != nil {
			slog.Error("Unable to read plugin info. Skipped", "err", err.Error())
		} else {
			size := fileInfo.Size()
			fileMode := fileInfo.Mode()
			isExecutable := fileMode&0100 != 0
			isFile := !entry.IsDir()
			if isFile && isExecutable && size > 0 {
				count++
				pluginInfo, found := svc.plugins[entry.Name()]
				if !found {
					pluginInfo = &runapi.PluginInfo{
						Name:    entry.Name(),
						Path:    path.Join(folder, entry.Name()),
						Uptime:  0,
						Running: false,
					}
					svc.plugins[pluginInfo.Name] = pluginInfo
				}
				pluginInfo.ModifiedTime = fileInfo.ModTime().Format(time.RFC3339)
				pluginInfo.Size = size
			}
		}
	}
	slog.Info("found plugins", "count", count, "directory", folder)
	return nil
}

// List all available or just the running plugins and their status
// This returns the list of plugins sorted by name
func (svc *LauncherService) List(
	ctx clidone.ServiceContext, args runapi.ListArgs) (runapi.ListResp, error) {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	// get the keys of the plugins to include and sort them
	keys := make([]string, 0, len(svc.plugins))
	for key, val := range svc.plugins {
		if !args.OnlyRunning || val.Running {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	infoList := make([]runapi.PluginInfo, 0, len(keys))
	for _, key := range keys {
		svcInfo := svc.plugins[key]
		svc.updateStatus(svcInfo)
		infoList = append(infoList, *svcInfo)
	}
	resp := runapi.ListResp{PluginInfoList: infoList}
	return resp, nil
}

// ScanPlugins scans the plugin folder for changes and updates the plugins list
func (svc *LauncherService) ScanPlugins() error {
	svc.mux.Lock()
	defer svc.mux.Unlock()
	//// include the core
	//err := svc.addCore()
	//if err != nil {
	//	slog.Error(err.Error())
	//	return err
	//}
	// add plugins
	err := svc.addPlugins(svc.env.PluginsDir)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	return nil
}

// Start the run service
// This first starts the core defined in the config, then connects to the hub
// to be able to create auth keys and tokens, and to subscribe to rpc requests.
//
// Call stop to end
func (svc *LauncherService) Start() error {
	slog.Warn("Starting LauncherService", "clientID", svc.env.ClientID)
	svc.isRunning.Store(true)

	// include the core message server
	coreBin := svc.cfg.CoreBin
	if coreBin != "" {
		err := svc.addCore(coreBin)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
	}
	// 1: determine the inventory of plugins
	_ = svc.WatchPlugins()
	err := svc.ScanPlugins()
	if err != nil {
		return err
	}

	// 2: start the core, if configured
	svc.mux.Lock()
	_, foundCore := svc.plugins[coreBin]
	svc.mux.Unlock()
	if foundCore {
		// core is added and starts first
		_, err = svc._startPlugin(coreBin)
		if err != nil {
			slog.Error("Starting core failed", "coreBin", coreBin, "err", err)
			return err
		} else {
			slog.Warn("core started successfully", "coreBin", coreBin)

		}
	}

	// 3: a connection to the message bus is needed
	if svc.hc == nil {
		svc.hc, err = clidone.ConnectToHub(
			svc.env.ServerURL, svc.env.ClientID, svc.env.CertsDir, "", "")
		if err != nil {
			err = fmt.Errorf("failed starting run service: %w", err)
			return err
		}
	}

	// the auth service is used to create plugin credentials
	svc.mngAuth = authcli.NewManageClients(svc.hc)

	// start listening to requests
	//svc.mngSub, err = svc.hc.SubRPCRequest(run.ManageCapability, svc.HandleRequest)
	svc.hc.SetRPCCapability(runapi.ManageCapability,
		map[string]interface{}{
			runapi.ListMethod:            svc.List,
			runapi.StartPluginMethod:     svc.StartPlugin,
			runapi.StartAllPluginsMethod: svc.StartAllPlugins,
			runapi.StopPluginMethod:      svc.StopPlugin,
			runapi.StopAllPluginsMethod:  svc.StopAllPlugins,
		})

	// 4: autostart the configured 'autostart' plugins
	// Log errors but do not stop the run
	for _, name := range svc.cfg.Autostart {
		_, _ = svc._startPlugin(name)
	}
	return err
}

// Stop the run and all running plugins
func (svc *LauncherService) Stop() error {
	slog.Warn("Stopping run service")
	svc.isRunning.Store(false)
	err := svc.StopAllPlugins(clidone.ServiceContext{},
		&runapi.StopAllPluginsArgs{IncludingCore: true})
	return err
}

// WatchPlugins watches the bin and plugins folder for changes and reloads
// This will detect adding new plugins without requiring a restart.
func (svc *LauncherService) WatchPlugins() error {
	svc.serviceWatcher, _ = fsnotify.NewWatcher()
	err := svc.serviceWatcher.Add(svc.env.BinDir)
	if err == nil && svc.env.PluginsDir != "" {
		err = svc.serviceWatcher.Add(svc.env.PluginsDir)
	}
	if err == nil {
		go func() {
			for {
				select {
				case <-svc.done:
					slog.Info("service watcher ended")
					return
				case event := <-svc.serviceWatcher.Events:
					isRunning := svc.isRunning.Load()
					if isRunning {
						slog.Info("watcher event", "event", event)
						_ = svc.ScanPlugins()
					} else {
						slog.Info("service watcher stopped")
						return
					}
				case err := <-svc.serviceWatcher.Errors:
					slog.Error("error", "err", err)
				}
			}
		}()

	}
	return err
}

// NewLauncherService returns a new run instance for the plugins in the given plugins folder.
// This scans the folder for executables, adds these to the list of available plugins and autostarts plugins
// Logging will be enabled based on LauncherConfig.
//
// The hub client is intended when an existing message bus is used. If the core is
// started by the run then it is ignored.
func NewLauncherService(
	env plugin.AppEnvironment,
	cfg runcfg.LauncherConfig,
	hc *clidone.HubClient,
) *LauncherService {

	ls := &LauncherService{
		env:     env,
		cfg:     cfg,
		plugins: make(map[string]*runapi.PluginInfo),
		cmds:    make([]*exec.Cmd, 0),
		hc:      hc,
	}

	return ls
}
