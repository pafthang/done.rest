package about

import (
	"net/http"

	app "github.com/hiveot/hub/done_mod/mod_web/web_view/app"
)

const TemplateFile = "about.gohtml"

func RenderAbout(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}
	app.RenderAppOrFragment(w, r, TemplateFile, data)
}
