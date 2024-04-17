package directory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	dircli "github.com/hiveot/hub/done_mod/mod_dir/dir_cli"
	websession "github.com/hiveot/hub/done_mod/mod_web/web_session"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/app"
	"github.com/hiveot/hub/done_tool/things"
)

const DirectoryTemplate = "directory.gohtml"

type DirGroup struct {
	AgentID string
	Things  []*things.TD
}

type DirectoryData struct {
	Groups map[string]*DirGroup
}

// Sort the given list of things and group them by publishing agent
// this returns a map of groups each containing an array of thing values
func sortByPublisher(tvList []things.ThingValue) *DirectoryData {
	dirData := &DirectoryData{
		Groups: make(map[string]*DirGroup),
	}

	// sort by agent+thingID for now
	sort.Slice(tvList, func(i, j int) bool {
		item1 := tvList[i]
		item2 := tvList[j]
		return item1.AgentID+item1.ThingID < item2.AgentID+item2.ThingID
	})
	for _, tv := range tvList {
		tplGroup, found := dirData.Groups[tv.SenderID]
		if !found {
			tplGroup = &DirGroup{
				AgentID: tv.SenderID,
				Things:  make([]*things.TD, 0),
			}
			dirData.Groups[tv.SenderID] = tplGroup
		}
		td := things.TD{}
		err := json.Unmarshal(tv.Data, &td)
		if err == nil {
			tplGroup.Things = append(tplGroup.Things, &td)
			if len(tplGroup.Things) == 0 {
				slog.Error("append failed")
			}
		}
	}
	return dirData
}

// RenderDirectory renders the directory of Things.
//
// This supports both a full and fragment rendering.
// Fragment rendering using htmx must use the #directory target.
// To view the directory, the #directory hash must be included at the end of the URL.
// E.g.: /directory/#directory
func RenderDirectory(w http.ResponseWriter, r *http.Request) {
	var data = make(map[string]any)

	// 1: get session
	mySession, err := websession.GetSessionFromContext(r)
	if err == nil {
		hc := mySession.GetHubClient()
		rd := dircli.NewReadDirectoryClient(hc)
		thingsList, err2 := rd.GetTDs(0, 100)
		err = err2
		if err == nil {
			dirGroups := sortByPublisher(thingsList)
			data["Directory"] = dirGroups
		} else {
			// the 'Directory' attribute is used by html know if to reload
			err = fmt.Errorf("unable to load directory: %w", err)
			slog.Error(err.Error())
		}
	}
	if err != nil {
		slog.Info("failed getting session. Redirecting to login", "err", err.Error())
		// assume this is an auth issue, maybe the browser was still open or a bookmark was used
		//mySession.Close()
		//http.Error(w, err.Error(), http.StatusUnauthorized)
		// FIXME: logout doesn't update URL to /login (need navigateto?)
		websession.SessionLogout(w, r)
		return
	}
	data["PageNr"] = 1

	// full render or fragment render
	app.RenderAppOrFragment(w, r, DirectoryTemplate, data)
}
