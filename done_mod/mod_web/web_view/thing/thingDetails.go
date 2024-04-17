package thing

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	apigo "github.com/hiveot/hub/done_api/api_go"
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	dircli "github.com/hiveot/hub/done_mod/mod_dir/dir_cli"
	histcli "github.com/hiveot/hub/done_mod/mod_hist/hist_cli"
	websession "github.com/hiveot/hub/done_mod/mod_web/web_session"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/app"
	"github.com/hiveot/hub/done_tool/things"
)

const TemplateFile = "thingDetails.gohtml"

type DetailsTemplateData struct {
	AgentID    string
	ThingID    string
	MakeModel  string
	Name       string
	DeviceType string
	TD         things.TD
	// These lists are sorted by property/event/action name
	Attributes map[string]*things.PropertyAffordance
	Config     map[string]*things.PropertyAffordance
	Values     things.ThingValueMap
}

// return a map with the latest property values of a thing or nil if failed
func getLatest(agentID string, thingID string, hc *clidone.HubClient) (things.ThingValueMap, error) {
	data := things.NewThingValueMap()
	rh := histcli.NewReadHistoryClient(hc)
	tvs, err := rh.GetLatest(agentID, thingID, nil)
	if err != nil {
		return data, err
	}
	for _, tv := range tvs {
		data.Set(tv.Name, tv)
		if tv.Data == nil {
			tv.Data = []byte("")
		}
	}
	//_ = data.of("")
	return data, nil
}

// RenderThingDetails renders thing details view fragment 'thingDetails.html'
// URL parameters:
// @param agentID of the publisher
// @param thingID to view
func RenderThingDetails(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]any)
	agentID := chi.URLParam(r, "agentID")
	thingID := chi.URLParam(r, "thingID")
	thingData := &DetailsTemplateData{
		Attributes: make(map[string]*things.PropertyAffordance),
		Config:     make(map[string]*things.PropertyAffordance),
	}
	thingData.ThingID = thingID
	thingData.AgentID = agentID
	data["Thing"] = thingData
	data["Title"] = "details of thing"

	mySession, err := websession.GetSessionFromContext(r)
	if err == nil {
		hc := mySession.GetHubClient()
		rd := dircli.NewReadDirectoryClient(hc)
		tv, err2 := rd.GetTD(agentID, thingID)
		err = err2
		if err == nil {
			err = json.Unmarshal(tv.Data, &thingData.TD)
			// split properties into attributes and configuration
			for k, prop := range thingData.TD.Properties {
				if prop.ReadOnly {
					thingData.Attributes[k] = prop
				} else {
					thingData.Config[k] = prop
				}
			}

			// get the latest values if available
			propMap, err2 := getLatest(agentID, thingID, hc)
			err = err2
			thingData.Values = propMap
			thingData.DeviceType = thingData.TD.AtType

			// get the value of a make & model properties, if they exist
			// TODO: this is a bit of a pain to do. Is this a common problem?
			makeID, _ := thingData.TD.GetPropertyOfType(apigo.PropDeviceMake)
			modelID, _ := thingData.TD.GetPropertyOfType(apigo.PropDeviceModel)
			makeValue := propMap.Get(makeID)
			modelValue := propMap.Get(modelID)
			if makeValue != nil {
				thingData.MakeModel = string(makeValue.Data) + ", "
			}
			if modelValue != nil {
				thingData.MakeModel = thingData.MakeModel + string(modelValue.Data)
			}
			// use name from configuration if available. Fall back to title.
			thingData.Name = thingData.Values.ToString(apigo.PropDeviceTitle)
			if thingData.Name == "" {
				thingData.Name = thingData.TD.Title
			}
		}
	}
	if err != nil {
		slog.Error("Failed loading Thing info",
			"agentID", agentID, "thingID", thingID, "err", err.Error())
	}
	// full render or fragment render
	app.RenderAppOrFragment(w, r, TemplateFile, data)
}
