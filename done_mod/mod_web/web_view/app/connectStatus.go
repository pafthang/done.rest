package app

import (
	"net/http"

	"github.com/hiveot/hub/done_cli/cli_done/transport"
	websession "github.com/hiveot/hub/done_mod/mod_web/web_session"
	webview "github.com/hiveot/hub/done_mod/mod_web/web_view"
)

const ConnectStatusTemplate = "connectStatus.gohtml"

// ConnectStatus describes the message bus connection status of the current session
type ConnectStatus struct {
	// the login ID which is used to connect
	LoginID string
	// description of the connection status
	Description string
	// mdi icon set icon name representing the status
	IconName string
	// optional error text if connection failed
	Error string
	// simple flag whether a connection is established
	IsConnected bool
}

// GetConnectStatus returns the description of the connection status
func GetConnectStatus(r *http.Request) *ConnectStatus {
	cs, _ := websession.GetSessionFromContext(r)
	status := &ConnectStatus{
		IconName:    "link-off",
		Description: "disconnected",
		IsConnected: false,
		Error:       "",
	}
	if cs == nil {
		status.Description = "Session not established"
	} else {
		cStat := cs.GetStatus()
		status.LoginID = cs.GetHubClient().ClientID()
		if cStat.LastError != nil {
			status.Error = cStat.LastError.Error()
		}
		if cStat.ConnectionStatus == transport.Connected {
			status.IconName = "link"
			status.Description = "Connected to the Hub"
			status.IsConnected = true
		} else if cStat.ConnectionStatus == transport.ConnectFailed {
			status.IconName = "link-off"
			status.Description = "Connection failed"
		} else if cStat.ConnectionStatus == transport.Connecting {
			status.IconName = "leak-off"
			status.Description = "Reconnecting"
		} else {
			status.IconName = "link-off"
			status.Description = "unknown"
		}
	}
	return status
}

// RenderConnectStatus renders the presentation of the client connection to the Hub message bus.
// This only renders the fragment. On a full page refresh this renders inside the base.html
func RenderConnectStatus(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}
	status := GetConnectStatus(r)
	data["Status"] = status

	// render with base or as fragment
	webview.TM.RenderTemplate(w, r, ConnectStatusTemplate, data)
}
