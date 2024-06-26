package provcli

import (
	"fmt"
	"log/slog"
	"time"

	provapi "github.com/hiveot/hub/done_mod/mod_prov/prov_api"
	"github.com/hiveot/hub/done_tool/discovery"
)

// DiscoverIdProvServer attempts to discover the provisioning server
// This returns its URL, the server core used, or an error if not found
func DiscoverIdProvServer() (url string, core string, error error) {
	address, port, params, records, err := discovery.DiscoverService(
		provapi.ProvisioningServerType, time.Second*3, true)

	if err != nil {
		// failed, nothing to be found
		slog.Warn("DiscoverIdProvServer: not found")
		return "", "", err
	}
	core = params["core"]
	url, found := params["url"]
	if !found {
		url = fmt.Sprintf("https://%s:%d%s", address, port, params["path"])
	}
	slog.Info("DiscoverIdProvServer",
		slog.Int("Nr records", len(records)),
		slog.String("url", url),
		slog.String("core", core))
	return url, core, err

}
