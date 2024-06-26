package websrv

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	vocab "github.com/hiveot/hub/done_api/api_go"
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	"github.com/hiveot/hub/done_cli/cli_done/transport"

	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	authcli "github.com/hiveot/hub/done_mod/mod_auth/auth_cli"
	modweb "github.com/hiveot/hub/done_mod/mod_web"
	webapi "github.com/hiveot/hub/done_mod/mod_web/web_api"
	websession "github.com/hiveot/hub/done_mod/mod_web/web_session"
	webview "github.com/hiveot/hub/done_mod/mod_web/web_view"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/about"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/app"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/dashboard"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/directory"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/login"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/status"
	"github.com/hiveot/hub/done_mod/mod_web/web_view/thing"
	"github.com/hiveot/hub/done_tool/things"
)

// HiveovService operates the html web server.
// It utilizes gin, htmx and TempL for serving html.
// credits go to: https://github.com/marco-souza/gx/blob/main/cmd/server/server.go
type HiveovService struct {
	port         int  // listening port
	dev          bool // development configuration
	shouldUpdate bool
	router       chi.Router
	// filesystem location of the ./static, webcomp, and ./views template root folder
	rootPath string
	tm       *webview.TemplateManager

	// hc hub client of this service.
	// This client's CA and URL is also used to establish client sessions.
	hc *clidone.HubClient

	// cookie signing
	signingKey *ecdsa.PrivateKey

	// run in debug mode, extra logging and reload templates render
	debug bool
}

// setup the chain of routes used by the service and return the router
// rootPath points to the filesystem containing /static and template files
func (svc *HiveovService) createRoutes(rootPath string) http.Handler {
	var staticFileServer http.Handler

	if rootPath == "" {
		staticFileServer = http.FileServer(
			&StaticFSWrapper{
				FileSystem:   http.FS(modweb.EmbeddedStatic),
				FixedModTime: time.Now(),
			})
	} else {
		// during development when run from the 'hub' project directory
		staticFileServer = http.FileServer(http.Dir(rootPath))
	}
	router := chi.NewRouter()

	// TODO: add csrf support in posts
	//csrfMiddleware := csrf.Protect(
	//	[]byte("32-byte-long-auth-key"),
	//	csrf.SameSite(csrf.SameSiteStrictMode))

	//-- add the routes and middleware
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	//router.Use(csrfMiddleware)
	router.Use(middleware.Compress(5,
		"text/html", "text/css", "text/javascript", "image/svg+xml"))

	//--- public routes do not require a Hub connection
	router.Group(func(r chi.Router) {
		// serve static files with the startup timestamp so caching works
		//staticFileServer := http.FileServer(
		//	&StaticFSWrapper{
		//		FileSystem:   http.FS(staticFS),
		//		FixedModTime: time.Now(),
		//	})

		// full page routes
		r.Get("/static/*", staticFileServer.ServeHTTP)
		r.Get("/webcomp/*", staticFileServer.ServeHTTP)
		r.Get("/login", login.RenderLogin)
		r.Post("/login", login.PostLogin)
		r.Get("/logout", websession.SessionLogout)

		// sse has its own validation instead of using session context (which reconnects or redirects to /login)
		r.Get("/sse", websession.SseHandler)

	})

	//--- private routes that requires a valid session
	router.Group(func(r chi.Router) {
		// these routes must be authenticated otherwise redirect to /login
		r.Use(websession.AddSessionToContext())

		// see also:https://medium.com/gravel-engineering/i-find-it-hard-to-reuse-root-template-in-go-htmx-so-i-made-my-own-little-tools-to-solve-it-df881eed7e4d
		// these renderer full page or fragments for non hx-boost hx-requests
		r.Get("/", app.RenderApp)
		r.Get("/app/about", about.RenderAbout)
		r.Get("/app/connectStatus", app.RenderConnectStatus)
		r.Get("/app/dashboard", dashboard.RenderDashboard)
		r.Get("/app/dashboard/{page}", dashboard.RenderDashboard) // TODO: support multiple pages
		r.Get("/app/directory", directory.RenderDirectory)
		r.Get("/app/thing/{agentID}/{thingID}", thing.RenderThingDetails)
		r.Get("/app/thing/editConfig", thing.RenderEditThingConfig)
		r.Post("/app/thing/{agentID}/{thingID}/{propKey}", thing.PostThingConfig)
		r.Get("/app/status", status.RenderStatus)
	})

	return router
}

// CreateHiveoviewTD creates a new Thing TD document describing the service capability
func (svc *HiveovService) CreateHiveoviewTD() *things.TD {
	title := "Web Server"
	deviceType := vocab.ThingService
	td := things.NewTD(webapi.HiveoviewServiceID, title, deviceType)
	// TODO: add properties: uptime, max nr clients

	td.AddEvent("activeSessions", "", "Nr Sessions", "Number of currently active sessions",
		&things.DataSchema{
			//AtType: vocab.SessionCount,
			Type: vocab.WoTDataTypeInteger,
		})

	return td
}

// Start the web server and publish the service's own TD.
func (svc *HiveovService) Start(hc *clidone.HubClient) error {
	slog.Warn("Starting HiveovService", "clientID", hc.ClientID())
	svc.hc = hc

	// publish a TD for each service capability and set allowable roles
	// in this case only a management capability is published
	myProfile := authcli.NewProfileClient(svc.hc)
	err := myProfile.SetServicePermissions(webapi.HiveoviewServiceID, []string{
		authapi.ClientRoleAdmin,
		authapi.ClientRoleService})
	if err != nil {
		slog.Error("failed to set the hiveoview service permissions", "err", err.Error())
	}

	myTD := svc.CreateHiveoviewTD()
	myTDJSON, _ := json.Marshal(myTD)
	err = svc.hc.PubEvent(webapi.HiveoviewServiceID, transport.EventNameTD, myTDJSON)
	if err != nil {
		slog.Error("failed to publish the hiveoview service TD", "err", err.Error())
	}

	// Setup the handling of incoming web sessions
	sm := websession.GetSessionManager()
	connStat := hc.GetStatus()
	sm.Init(connStat.HubURL, svc.signingKey, connStat.CaCert, svc.hc.ClientKP())

	// parse the templates
	svc.tm.ParseAllTemplates()

	// add the routes
	router := svc.createRoutes(svc.rootPath)

	// TODO: change into TLS using a signed server certificate
	addr := fmt.Sprintf(":%d", svc.port)
	go func() {
		err = http.ListenAndServe(addr, router)
		if err != nil {
			// TODO: close gracefully
			slog.Error("Failed starting server", "err", err)
			// service must exit on close
			time.Sleep(time.Second)
			os.Exit(0)
		}
	}()
	return nil
}

func (svc *HiveovService) Stop() {
	// TODO: send event the service has stopped
	svc.hc.Disconnect()
	//svc.router.Stop()

	//if err != nil {
	//	slog.Error("Stop error", "err", err)
	//}
}

// NewHiveovService creates a new service instance that serves the
// content from a http.FileSystem.
//
// rootPath is the root directory when serving files from the filesystem.
// This must contain static/, views/ and webc/ directories.
// If empty, the embedded filesystem is used.
//
// serverPort is the port of the web server will listen on
// debug to enable debugging output
// signingKey used to sign cookies. Using nil means that a server restart will invalidate the cookies
// rootPath
func NewHiveovService(serverPort int, debug bool,
	signingKey *ecdsa.PrivateKey, rootPath string,
) *HiveovService {
	templatePath := rootPath
	if rootPath != "" {
		templatePath = path.Join(rootPath, "views")
	}
	if signingKey == nil {
		signingKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	tm := webview.InitTemplateManager(templatePath)
	svc := HiveovService{
		port:         serverPort,
		shouldUpdate: true,
		debug:        debug,
		signingKey:   signingKey,
		rootPath:     rootPath,
		tm:           tm,
	}
	return &svc
}
