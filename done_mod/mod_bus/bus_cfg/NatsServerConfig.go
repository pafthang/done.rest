package buscfg

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path"

	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	"github.com/hiveot/hub/done_tool/certs"
	"github.com/hiveot/hub/done_tool/keys"
	"github.com/hiveot/hub/done_tool/net"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
)

const NoAuthUserID = "unauthenticated"

// NatsServerConfig holds the configuration for nkeys and jwt based servers
type NatsServerConfig struct {
	Host            string            `yaml:"host,omitempty"`            // default: localhost
	Port            int               `yaml:"port,omitempty"`            // default: 4222
	WSPort          int               `yaml:"wsPort,omitempty"`          // default: 0 (disabled)
	LogLevel        string            `yaml:"logLevel,omitempty"`        // default: warn
	LogFile         string            `yaml:"logFile,omitempty"`         // default: no logfile
	Debug           bool              `yaml:"debug,omitempty"`           // default: false
	MaxDataMemoryMB int               `yaml:"maxDataMemoryMB,omitempty"` // default: 1024
	DataDir         string            `yaml:"dataDir,omitempty"`         // default is server default
	AppAccountName  string            `yaml:"appAccountName,omitempty"`  // default: hiveot
	NoAutoStart     bool              `yaml:"noAutoStart,omitempty"`
	CaCert          *x509.Certificate `yaml:"-"` // preset, load, or error
	CaKey           keys.IHiveKey     `yaml:"-"` // preset, load, or error
	ServerTLS       *tls.Certificate  `yaml:"-"` // preset, load, or generate
	AppAccountKP    nkeys.KeyPair     `yaml:"-"` // preset, load, or generate
	AdminUserKP     nkeys.KeyPair     `yaml:"-"` // generated
	CoreServiceKP   nkeys.KeyPair     `yaml:"-"` // generated
	SystemAccountKP nkeys.KeyPair     `yaml:"-"` // generated
	SystemUserKP    nkeys.KeyPair     `yaml:"-"` // generated
	AppAcct         *server.Account   `yaml:"-"`
}

// Setup the nats server config.
// This applies sensible defaults to config.
//
// Any existing values that are previously set remain unchanged.
// Missing values are created.
// Certs and keys are loaded as per configuration.
//
// Set 'writeChanges' to persist generated server cert, operator and account keys
//
//		keysDir is the default key location
//		storesDir is the data storage root (default $HOME/stores)
//	 writeChanges writes generated account key to the keysDir
func (cfg *NatsServerConfig) Setup(keysDir, storesDir string, writeChanges bool) (err error) {

	// Step 1: Apply defaults parameters
	if cfg.Host == "" {
		outboundIP := net.GetOutboundIP("")
		cfg.Host = outboundIP.String()
	}
	if cfg.Port == 0 {
		cfg.Port = 4222
	}
	if cfg.WSPort == 0 {
		//appCfg.WSPort = 8222
	}
	if cfg.DataDir == "" {
		cfg.DataDir = path.Join(storesDir, "natsserver")
	}
	if cfg.MaxDataMemoryMB == 0 {
		cfg.MaxDataMemoryMB = 1024
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "warn"
	}
	if cfg.AppAccountName == "" {
		cfg.AppAccountName = "hiveot"
	}

	// Step 2: generate missing certificates
	// These are typically set directly before running setup so this is intended
	// for testing.
	if cfg.CaCert == nil || cfg.CaKey == nil {
		cfg.CaCert, cfg.CaKey, _ = certs.CreateCA("hiveot", 365)
	}
	if cfg.ServerTLS == nil && cfg.CaKey != nil {
		serverKeys := keys.NewKey(cfg.CaKey.KeyType()) // use same type for key as the CA
		names := []string{cfg.Host}
		serverX509, err := certs.CreateServerCert(
			cfg.AppAccountName, "server",
			365, // validity matches the CA
			serverKeys,
			names, cfg.CaCert, cfg.CaKey)
		if err != nil {
			slog.Error("unable to generate server cert. Not using TLS.", "err", err)
		} else {
			cfg.ServerTLS = certs.X509CertToTLS(serverX509, serverKeys)
		}
	}

	// Step 3: Load or generate Account key
	if cfg.AppAccountKP == nil {
		// load/create an account key (not a user key)
		kpPath := path.Join(keysDir, cfg.AppAccountName+"App.key")
		if !path.IsAbs(kpPath) {
			kpPath = path.Join(keysDir, kpPath)
		}
		kpSeed, err := os.ReadFile(kpPath)
		if err == nil {
			cfg.AppAccountKP, err = nkeys.ParseDecoratedNKey(kpSeed)
		}
		if err != nil {
			slog.Warn("Generating app account key.")
			cfg.AppAccountKP, _ = nkeys.CreateAccount()
			if writeChanges {
				kpSeed, _ := cfg.AppAccountKP.Seed()
				err = os.WriteFile(kpPath, kpSeed, 0400)
				if err != nil {
					return fmt.Errorf("failed to persist app account key: %w", err)
				}
			}
		}
	}

	// Step 4: generate derived keys
	if cfg.AdminUserKP == nil {
		cfg.AdminUserKP, _ = cfg.LoadCreateUserKP(authapi.DefaultAdminUserID, keysDir, writeChanges)
	}
	if cfg.CoreServiceKP == nil {
		cfg.CoreServiceKP, _ = nkeys.CreateUser()
	}
	if cfg.SystemAccountKP == nil {
		cfg.SystemAccountKP, _ = nkeys.CreateAccount()
	}
	if cfg.SystemUserKP == nil {
		cfg.SystemUserKP, _ = cfg.LoadCreateUserKP(cfg.AppAccountName+"System", keysDir, writeChanges)
	}

	return nil
}

// CreateNatsNKeyOptions create a Nats options struct for use with NKey authentication.
// Note that Setup() must have been called first.
func (cfg *NatsServerConfig) CreateNatsNKeyOptions() (server.Options, error) {
	natsOpts := server.Options{}
	tmpFile := path.Join(os.TempDir(), "natsserver.conf")

	// create the config to load
	// Frustratingly, this is the only way to enable jetstream on an account that persists after options reload
	cfgContent := ` 
accounts { 
	` + cfg.AppAccountName + `: {
		jetstream: enabled
	}
}`

	err := os.WriteFile(tmpFile, []byte(cfgContent), 0600)
	if err != nil {
		return natsOpts, err
	}

	// load the file
	err = natsOpts.ProcessConfigFile(tmpFile)
	natsOpts.ConfigFile = "" // it was just temporary
	_ = os.Remove(tmpFile)
	if err != nil {
		return natsOpts, err
	}
	natsOpts.Host = cfg.Host
	natsOpts.Port = cfg.Port

	systemAcct := server.NewAccount("SYS")
	systemAccountPub, _ := cfg.SystemAccountKP.PublicKey()
	systemAcct.Nkey = systemAccountPub
	natsOpts.SystemAccount = "SYS"

	// NewAccount creates a limitless account. There is no way to set a limit though :/
	cfg.AppAcct = server.NewAccount(cfg.AppAccountName)
	appAccountPub, _ := cfg.AppAccountKP.PublicKey()
	cfg.AppAcct.Nkey = appAccountPub

	natsOpts.Accounts = append(natsOpts.Accounts, systemAcct)

	coreServicePub, _ := cfg.CoreServiceKP.PublicKey()
	systemUserPub, _ := cfg.SystemUserKP.PublicKey()
	natsOpts.Nkeys = []*server.NkeyUser{
		{
			Nkey:        coreServicePub,
			Permissions: nil, // unlimited
			Account:     cfg.AppAcct,
		}, {
			Nkey:    systemUserPub,
			Account: systemAcct,
		},
	}

	natsOpts.Users = []*server.User{}
	natsOpts.JetStream = true
	natsOpts.JetStreamMaxMemory = int64(cfg.MaxDataMemoryMB) * 1024 * 1024
	natsOpts.StoreDir = cfg.DataDir

	// logging
	natsOpts.Debug = cfg.Debug
	natsOpts.Logtime = true

	if cfg.CaCert != nil && cfg.ServerTLS != nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AddCert(cfg.CaCert)
		clientCertList := []tls.Certificate{*cfg.ServerTLS}
		tlsConfig := &tls.Config{
			ServerName:   "HiveOT Hub",
			ClientCAs:    caCertPool,
			RootCAs:      caCertPool,
			Certificates: clientCertList,
			ClientAuth:   tls.VerifyClientCertIfGiven,
			MinVersion:   tls.VersionTLS13,
		}
		natsOpts.AuthTimeout = 101 // for debugging auth
		natsOpts.TLSTimeout = 100  // for debugging auth
		natsOpts.TLSConfig = tlsConfig
	}
	return natsOpts, err
}

// LoadCreateUserKP loads a user keypair, or creates one if it doesn't exist
// By convention the filenam is {clientID}.key
//
//	clientID is the serviceID/deviceID/userID
//	writeChanges if a file is given and key is generated
func (cfg *NatsServerConfig) LoadCreateUserKP(clientID string, keysDir string, writeChanges bool) (userKP nkeys.KeyPair, err error) {
	// attempt to load
	kpPath := path.Join(keysDir, clientID+".key")
	kpSeed, err := os.ReadFile(kpPath)
	if err == nil {
		userKP, err = nkeys.ParseDecoratedNKey(kpSeed)
	}

	// no key file, create and save
	if userKP == nil {
		err = nil
		userKP, _ = nkeys.CreateUser()
		slog.Info("LoadCreateUserKP Keys not found. Creating new keys",
			slog.String("kpPath", kpPath),
			slog.Bool("writeChanges", writeChanges))
		if writeChanges {
			kpSeed, _ := userKP.Seed()
			err = os.WriteFile(kpPath, kpSeed, 0400)
		}
	}
	return userKP, err
}
