package authcfg

import (
	"path"

	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
)

// AuthConfig contains the auth service configuration
type AuthConfig struct {
	PasswordFile             string `yaml:"passwordFile,omitempty"`
	DeviceTokenValidityDays  int    `yaml:"deviceTokenValidityDays,omitempty"`
	ServiceTokenValidityDays int    `yaml:"serviceTokenValidityDays,omitempty"`
	UserTokenValidityDays    int    `yaml:"userTokenValidityDays,omitempty"`
	NoAutoStart              bool   `yaml:"noAutoStart,omitempty"`
	KeysDir                  string `yaml:"certsDir,omitempty"`
	AdminAccountID           string `yaml:"adminAccountID,omitempty"`
	LauncherAccountID        string `yaml:"launcherAccountID,omitempty"`
}

// Setup ensures config is valid
func (cfg *AuthConfig) Setup(keysDir, storesDir string) error {

	if cfg.PasswordFile == "" {
		cfg.PasswordFile = authapi.DefaultPasswordFile
	}
	if !path.IsAbs(cfg.PasswordFile) {
		cfg.PasswordFile = path.Join(storesDir, "auth", cfg.PasswordFile)
	}

	if cfg.DeviceTokenValidityDays == 0 {
		cfg.DeviceTokenValidityDays = authapi.DefaultDeviceTokenValidityDays
	}
	if cfg.ServiceTokenValidityDays == 0 {
		cfg.ServiceTokenValidityDays = authapi.DefaultServiceTokenValidityDays
	}
	if cfg.UserTokenValidityDays == 0 {
		cfg.UserTokenValidityDays = authapi.DefaultUserTokenValidityDays
	}
	cfg.KeysDir = keysDir
	cfg.AdminAccountID = authapi.DefaultAdminUserID
	cfg.LauncherAccountID = authapi.DefaultLauncherServiceID

	return nil
}
