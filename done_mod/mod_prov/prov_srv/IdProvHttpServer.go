package provsrv

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	provapi "github.com/hiveot/hub/done_mod/mod_prov/prov_api"
	"github.com/hiveot/hub/done_tool/tlsserver"
)

// IdProvHttpServer serves the provisioning requests
type IdProvHttpServer struct {
	tlsServer *tlsserver.TLSServer
	mng       *ManageIdProvService
}

// Stop the http server
func (srv *IdProvHttpServer) Stop() {
	if srv.tlsServer != nil {
		srv.tlsServer.Stop()
		srv.tlsServer = nil
	}
}

func (srv *IdProvHttpServer) handleRequest(w http.ResponseWriter, req *http.Request) {
	slog.Info("handleRequest", slog.String("remoteAddr", req.RemoteAddr))

	args := provapi.ProvisionRequestArgs{}
	data, err := io.ReadAll(req.Body)
	err2 := json.Unmarshal(data, &args)
	if err != nil || err2 != nil {
		slog.Warn("idprov handleRequest. bad request", "remoteAddr", req.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := clidone.ServiceContext{}
	resp, err := srv.mng.SubmitRequest(ctx, &args)
	if err != nil {
		slog.Warn("idprov handleRequest. refused", "err", err.Error())
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}
	respData, _ := json.Marshal(resp)
	_, err = w.Write(respData)
	if err != nil {
		slog.Error("error sending response", "err", err.Error(), "remoteAddr", req.RemoteAddr)
	}
}

// StartIdProvHttpServer starts the http server to handle provisioning requests
func StartIdProvHttpServer(
	port uint, serverCert *tls.Certificate, caCert *x509.Certificate, mng *ManageIdProvService) (*IdProvHttpServer, error) {

	tlsServer := tlsserver.NewTLSServer("", port, serverCert, caCert)
	srv := IdProvHttpServer{
		tlsServer: tlsServer,
		mng:       mng,
	}
	tlsServer.AddHandlerNoAuth(provapi.ProvisionRequestPath, srv.handleRequest)
	err := srv.tlsServer.Start()
	return &srv, err
}
