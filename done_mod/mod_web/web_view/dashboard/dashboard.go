package dashboard

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	websession "github.com/hiveot/hub/done_mod/mod_web/web_session"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/app"
)

const DashboardTemplate = "dashboard.gohtml"

// RenderDashboard renders the dashboard page or fragment
// This is intended for use from a htmx-get request with a target selector
func RenderDashboard(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]any)
	// when used with htmx, the URL contains the page to display
	pageName := chi.URLParam(r, "page")
	if pageName == "" {
		// when used without htmx there is no page, use the default page
		pageName = "default"
	}
	// TODO: load the dashboard tile configuration for the page name
	// use the session storage
	tiles := make([]websession.DashboardTile, 0)
	data["Dashboard"] = &websession.DashboardDefinition{
		Name:  pageName, // or use the default
		Tiles: tiles,
	}

	// full render or fragment render
	app.RenderAppOrFragment(w, r, DashboardTemplate, data)
}
