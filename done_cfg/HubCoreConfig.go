package donecfg

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"

	authcfg "github.com/hiveot/hub/done_mod/mod_auth/auth_cfg"
	buscfg "github.com/hiveot/hub/done_mod/mod_bus/bus_cfg"
	"github.com/hiveot/hub/done_tool/certs"
	"github.com/hiveot/hub/done_tool/keys"
	"github.com/hiveot/hub/done_tool/net"
	"github.com/hiveot/hub/done_tool/plugin"
	"gopkg.in/yaml.v3"
)

const DefaultServerCertFile = "hubCert.pem"
const DefaultServerKeyFile = "hubKey.pem"

// HubCoreConfig with core server, auth, cert and launcher configuration
// Used for launching the core.
// Use NewHubCoreConfig to create a default config
// FIXME: this is temporary, each service must handle their own config yaml
type HubCoreConfig struct {
	Env            plugin.AppEnvironment
	CaCertFile     string                  `yaml:"caCertFile"`     // default: caCert.pem
	CaKeyFile      string                  `yaml:"caKeyFile"`      // default: caKey.pem
	ServerCertFile string                  `yaml:"serverCertFile"` // default: hubCert.pem
	ServerKeyFile  string                  `yaml:"serverKeyFile"`  // default: hubKey.pem
	CaCert         *x509.Certificate       `yaml:"-"`              // preset, load, or error
	CaKey          keys.IHiveKey           `yaml:"-"`              // preset, load, or error
	ServerTLS      *tls.Certificate        `yaml:"-"`              // preset, load, or generate
	ServerKey      keys.IHiveKey           `yaml:"-"`
	NatsServer     buscfg.NatsServerConfig `yaml:"natsserver"`
	Auth           authcfg.AuthConfig      `yaml:"auth"`
	EnableMDNS     bool                    `yaml:"enableMDNS"`
}

// Setup ensures the hub core configuration exists along with certificate and key files.
// This:
// 1. If 'new' is true then delete existing config, certs, logs and storage.
// 2. Creates missing directories
// 3. Create missing certificates, including a self-signed CA.
// 4. Setup the message server config
// 5. Create auth keys for certs and launcher services
// 6. Create a default launcher config if none exists
//
//	env holds the application directory environment
//	new to initialize a new environment and delete existing data (careful!)
func (cfg *HubCoreConfig) Setup(env *plugin.AppEnvironment, new bool) error {
	var err error
	slog.Info("running setup",
		slog.Bool("--new", new),
		slog.String("home", env.HomeDir),
	)

	// 0: Load config file if given
	if _, err := os.Stat(cfg.Env.ConfigFile); err == nil {
		data, err := os.ReadFile(env.ConfigFile)
		if err != nil {
			return fmt.Errorf("unable to load config: %w", err)
		}
		err = yaml.Unmarshal(data, cfg)
		if err != nil {
			return fmt.Errorf("unable to parse config: %w", err)
		}
	}

	// 2: Setup directories
	err = cfg.setupDirectories(new)
	if err != nil {
		return err
	}

	// 3: Setup certificates
	cfg.CaCertFile = certs.DefaultCaCertFile
	cfg.CaKeyFile = certs.DefaultCaKeyFile
	cfg.ServerCertFile = DefaultServerCertFile
	cfg.ServerKeyFile = DefaultServerKeyFile
	cfg.setupCerts()
	// pass it on to the message server
	cfg.NatsServer.CaCert = cfg.CaCert
	cfg.NatsServer.CaKey = cfg.CaKey
	cfg.NatsServer.ServerTLS = cfg.ServerTLS

	// 4: Setup message server config

	cfg.setupNatsCore()

	// 5: setup authn config
	err = cfg.Auth.Setup(cfg.Env.CertsDir, cfg.Env.StoresDir)
	if err != nil {
		return err
	}

	// 6: setup launcher config
	//err = cfg.Launcher.Setup(cfg.Env.CertsDir)
	return err
}

// SetupCerts load or generate certificates.
// If certificates are preloaded then do nothing.
// If a CA doesn't exist then generate and save a new self-signed cert valid for localhost,127.0.0.1 and outbound IP
// The server certificates is always regenerated and saved.
// This panics if certs cannot be setup.
func (cfg *HubCoreConfig) setupCerts() {
	var err error
	certsDir := cfg.Env.CertsDir

	// setup files and folders
	if _, err = os.Stat(certsDir); err != nil {
		if err2 := os.MkdirAll(certsDir, 0755); err2 != nil && errors.Is(err, os.ErrExist) {
			errMsg := fmt.Errorf("unable to create certs directory '%s': %w", certsDir, err)
			panic(errMsg)
		}
	}

	caCertPath := cfg.CaCertFile
	if !path.IsAbs(caCertPath) {
		caCertPath = path.Join(certsDir, caCertPath)
	}
	caKeyPath := cfg.CaKeyFile
	if !path.IsAbs(caKeyPath) {
		caKeyPath = path.Join(certsDir, caKeyPath)
	}
	// 1: load the CA if available
	if cfg.CaCert == nil {
		slog.Info("loading CA certificate and key")
		cfg.CaCert, err = certs.LoadX509CertFromPEM(caCertPath)
	}
	// only load the ca key if the cert was loaded
	if cfg.CaCert != nil && cfg.CaKey == nil {
		cfg.CaKey, err = keys.NewKeyFromFile(caKeyPath)
	}

	// 2: if no CA exists, create it
	if err != nil || cfg.CaCert == nil || cfg.CaKey == nil {
		slog.Warn("creating a self-signed CA certificate and key", "caCertPath", caCertPath)

		cfg.CaCert, cfg.CaKey, err = certs.CreateCA("hiveot", 365*10)
		if err != nil {
			panic("Unable to create a CA cert: " + err.Error())
		}

		err = cfg.CaKey.ExportPrivateToFile(caKeyPath)
		if err == nil {
			err = certs.SaveX509CertToPEM(cfg.CaCert, caCertPath)
		}
		if err != nil {
			panic("Unable to save the CA cert or key: " + err.Error())
		}
	}

	// 3: Load or create a new server private key if it doesn't exist
	// As this key is used to sign tokens, save it after creation
	serverKeyPath := cfg.ServerKeyFile
	if !path.IsAbs(serverKeyPath) {
		serverKeyPath = path.Join(certsDir, serverKeyPath)
	}
	// load the server key if available
	if cfg.ServerKey == nil {
		slog.Warn("Loading server key", "serverKeyPath", serverKeyPath)
		cfg.ServerKey, err = keys.NewKeyFromFile(serverKeyPath)
	} else {
		slog.Warn("Using provided server key")
		err = nil
	}
	if err != nil || cfg.ServerKey == nil {
		slog.Warn("Creating server key")
		cfg.ServerKey = keys.NewKey(cfg.CaKey.KeyType()) // use same key type as CA
		err = cfg.ServerKey.ExportPrivateToFile(serverKeyPath)
		if err != nil {
			slog.Error("Unable to save the server key", "err", err)
		}
	}
	// create a new server cert
	serverCertPath := cfg.ServerCertFile
	if !path.IsAbs(serverCertPath) {
		serverCertPath = path.Join(certsDir, serverCertPath)
	}
	hostName, _ := os.Hostname()
	serverID := "nats-" + hostName
	ou := "hiveot"
	outboundIP := net.GetOutboundIP("")
	names := []string{"localhost", "127.0.0.1", hostName, outboundIP.String()}

	// regenerate a new server cert, valid for 1 year
	serverCert, err := certs.CreateServerCert(
		serverID, ou, 365, cfg.ServerKey, names, cfg.CaCert, cfg.CaKey)
	if err != nil {
		panic("Unable to create a server cert: " + err.Error())
	}
	cfg.ServerTLS = certs.X509CertToTLS(serverCert, cfg.ServerKey)

	slog.Warn("Writing server cert", "serverCertPath", serverCertPath)
	err = certs.SaveX509CertToPEM(serverCert, serverCertPath)
	if err != nil {
		slog.Error("writing server cert failed: ", "err", err)
	}
	//err = certs.SaveTLSCertToPEM(cfg.ServerTLS, serverCertPath, serverKeyPath)
}

// setupDirectories creates missing directories
// parameter new deletes existing data directories first. Careful!
func (cfg *HubCoreConfig) setupDirectories(new bool) error {
	// In a new environment, clear the home directory.
	// This is very destructive!
	// Do a sanity check on home first
	env := cfg.Env
	if path.Clean(env.HomeDir) == "/etc" {
		return fmt.Errorf("home cannot be /etc")
	} else if path.Clean(env.HomeDir) == "/tmp" {
		return fmt.Errorf("home cannot be /tmp. Choose a subdir")
	} else if path.Clean(path.Dir(env.HomeDir)) == "/home" {
		return fmt.Errorf("application home directory cannot be someone's home directory")
	}
	if _, err2 := os.Stat(env.HomeDir); err2 == nil && new {
		println("Setup new. Removing certs, stores and logs directories")
		// keep old config as there is no way to re-install defaults
		//_ = os.RemoveAll(env.ConfigDir)
		_ = os.RemoveAll(env.CertsDir)
		_ = os.RemoveAll(env.StoresDir)
		_ = os.RemoveAll(env.LogsDir)
	}

	if _, err2 := os.Stat(env.HomeDir); err2 != nil {
		err := os.MkdirAll(env.HomeDir, 0755)
		if err != nil {
			err = fmt.Errorf("unable to create home directory '%s': %w", env.HomeDir, err)
			return err
		}
	}
	// 2. ensure the directories exist
	if _, err2 := os.Stat(env.HomeDir); err2 != nil {
		_ = os.MkdirAll(env.HomeDir, 0755)
	}
	if _, err2 := os.Stat(env.BinDir); err2 != nil {
		_ = os.MkdirAll(env.BinDir, 0755)
	}
	if _, err2 := os.Stat(env.PluginsDir); err2 != nil {
		_ = os.MkdirAll(env.PluginsDir, 0755)
	}
	if _, err2 := os.Stat(env.CertsDir); err2 != nil {
		_ = os.MkdirAll(env.CertsDir, 0755)
	}
	if _, err2 := os.Stat(env.ConfigDir); err2 != nil {
		_ = os.MkdirAll(env.ConfigDir, 0755)
	}
	if _, err2 := os.Stat(env.LogsDir); err2 != nil {
		_ = os.MkdirAll(env.LogsDir, 0755)
	}
	if _, err2 := os.Stat(env.StoresDir); err2 != nil {
		_ = os.MkdirAll(env.StoresDir, 0755)
	}
	return nil
}

// setupNatsCore load or generate nats service and admin keys.
func (cfg *HubCoreConfig) setupNatsCore() error {
	var err error
	slog.Warn("setup nats core", "CertsDir", cfg.Env.CertsDir,
		"HomeDir", cfg.Env.HomeDir)
	cfg.NatsServer.CaCert = cfg.CaCert
	cfg.NatsServer.CaKey = cfg.CaKey
	cfg.NatsServer.ServerTLS = cfg.ServerTLS
	err = cfg.NatsServer.Setup(cfg.Env.CertsDir, cfg.Env.StoresDir, true)

	if cfg.NatsServer.DataDir == "" {
		panic("config is missing server data directory")
	}
	if _, err2 := os.Stat(cfg.NatsServer.DataDir); err2 != nil {
		slog.Warn("Creating server data directory: " + cfg.NatsServer.DataDir)
		err = os.MkdirAll(cfg.NatsServer.DataDir, 0700)
	}
	if err != nil {
		panic("error creating data directory: " + err.Error())
	}
	return err
}

// Load the core config from clidone.yaml
//func (cfg *HubCoreConfig) Load() error {
//	configFile := path.Join(cfg.Env.ConfigDir, HubCoreConfigFileName)
//	data, err := os.ReadFile(configFile)
//	if err != nil {
//		return err
//	}
//	err = yaml.Unmarshal(data, cfg)
//	return err
//}
//
//// Save the core config to clidone.yaml
//func (cfg *HubCoreConfig) Save() error {
//	configFile := path.Join(cfg.Env.ConfigDir, HubCoreConfigFileName)
//	data, err := yaml.Marshal(cfg)
//	if err != nil {
//		return err
//	}
//	err = os.WriteFile(configFile, data, 0644)
//	return err
//}

// NewHubCoreConfig creates a new configuration for the hub server and core services.
// Call Setup to load a config file and update directories.
func NewHubCoreConfig() *HubCoreConfig {
	return &HubCoreConfig{
		EnableMDNS: true,
	}
}
