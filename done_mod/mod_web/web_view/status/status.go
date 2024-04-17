package status

import (
	"net/http"

	"github.com/hiveot/hub/done_mod/mod_web/web_view/app"
)

const TemplateFile = "status.gohtml"

// RenderStatus renders the client status page
func RenderStatus(w http.ResponseWriter, r *http.Request) {
	status := app.GetConnectStatus(r)

	data := map[string]any{}
	data["Status"] = status

	// full render or fragment render
	app.RenderAppOrFragment(w, r, TemplateFile, data)
}
