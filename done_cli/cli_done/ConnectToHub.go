package clidone

import (
	"fmt"
	"log/slog"
	"path"
	"time"

	"github.com/hiveot/hub/done_tool/certs"
	"github.com/hiveot/hub/done_tool/discovery"
)

// ConnectToHub helper function to connect to the Hub using existing token and key files.
// This assumes that CA cert, user keys and auth token have already been set up and
// are available in the certDir.
// The key-pair file is named {certDir}/{clientID}.key
// The token file is named {certDir}/{clientID}.token
//
// 1. If no fullURL is given then use discovery to determine the URL
// 2. Determine the core to use
// 3. Load the CA cert
// 4. Create a hub client
// 5. Connect using token and key files
//
//	fullURL is the scheme://addr:port/[wspath] the server is listening on
//	clientID to connect as. Also used for the key and token file names
//	certDir is the location of the CA cert and key/token files
//	core optional core selection. Fallback is to auto determine based on URL.
//	 password optional for a user login
func ConnectToHub(fullURL string, clientID string, certDir string, core string, password string) (
	hc *HubClient, err error) {

	// 1. determine the actual address
	if fullURL == "" {
		// return after first result
		fullURL, _ = discovery.LocateHub(time.Second, true)
	}
	if clientID == "" {
		return nil, fmt.Errorf("missing clientID")
	}
	// 2. obtain the CA public cert to verify the server
	caCertFile := path.Join(certDir, certs.DefaultCaCertFile)
	caCert, err := certs.LoadX509CertFromPEM(caCertFile)
	if err != nil {
		return nil, err
	}
	// 3. Determine which core to use and setup the key and token filenames
	// By convention the key/token filename format is "{name}.key/{name}.token"
	hc = NewHubClient(fullURL, clientID, caCert)

	// 4. Connect and auth with token from file
	slog.Info("connecting to", "serverURL", fullURL)
	if password != "" {
		err = hc.ConnectWithPassword(password)
	} else {
		// login with token file
		err = hc.ConnectWithTokenFile(certDir)
	}
	if err != nil {
		return nil, err
	}
	return hc, err
}
