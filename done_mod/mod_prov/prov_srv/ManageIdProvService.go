package provsrv

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	authcli "github.com/hiveot/hub/done_mod/mod_auth/auth_cli"
	provapi "github.com/hiveot/hub/done_mod/mod_prov/prov_api"
)

type ManageIdProvService struct {

	// request status by deviceID
	// [deviceID] pub-key simple in-memory store
	requests map[string]provapi.ProvisionStatus

	//
	hc *clidone.HubClient
	// client of auth service used to create tokens
	authSvc *authcli.ManageClients
	// mutex to guard access to maps
	mux sync.RWMutex
}

// ApproveRequest approves an existing provisioning request.
// The client will be added on the next request.
// The next repeat request will return a short-lived token.
func (svc *ManageIdProvService) ApproveRequest(ctx clidone.ServiceContext,
	args *provapi.ApproveRequestArgs) error {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	slog.Info("ApproveRequest",
		slog.String("senderID", ctx.SenderID),
		slog.String("deviceID", args.ClientID))
	status, found := svc.requests[args.ClientID]
	if !found {
		return fmt.Errorf("provisioning request for device '%s' not found", args.ClientID)
	}
	status.Pending = false
	status.ClientType = args.ClientType
	status.ApprovedMSE = time.Now().UnixMilli()
	status.RejectedMSE = 0
	svc.requests[args.ClientID] = status
	return nil
}

// GetRequests returns list of requests since last start
// If args.OnlyPending is set then only return pending requests
// Note that rejected requests are never returned
func (svc *ManageIdProvService) GetRequests(ctx clidone.ServiceContext,
	args *provapi.GetRequestsArgs) (*provapi.GetRequestsResp, error) {
	svc.mux.RLock()
	defer svc.mux.RUnlock()

	resp := &provapi.GetRequestsResp{
		Requests: make([]provapi.ProvisionStatus, 0, len(svc.requests)),
	}
	for _, status := range svc.requests {
		if (status.Pending && args.Pending) ||
			(status.ApprovedMSE != 0 && args.Approved) ||
			(status.RejectedMSE != 0 && args.Rejected) {
			resp.Requests = append(resp.Requests, status)
		}
	}
	return resp, nil
}

// PreApproveClients uploads list of pre-approved devices and services
func (svc *ManageIdProvService) PreApproveClients(ctx clidone.ServiceContext,
	args *provapi.PreApproveClientsArgs) error {
	svc.mux.Lock()
	defer svc.mux.Unlock()
	slog.Info("PreApproveClients",
		slog.String("senderID", ctx.SenderID),
		slog.Int("count", len(args.Approvals)))

	for _, approval := range args.Approvals {
		if approval.ClientID == "" {
			slog.Warn("PreApproval of client without clientID", "clientID", ctx.SenderID)
		} else {
			svc.requests[approval.ClientID] = provapi.ProvisionStatus{
				ClientID:    approval.ClientID,
				ClientType:  approval.ClientType,
				PubKey:      approval.PubKey,
				MAC:         approval.MAC,
				Pending:     false,
				ApprovedMSE: time.Now().UnixMilli(),
			}
		}
	}
	return nil
}

// RejectRequest rejects a provisioning request
func (svc *ManageIdProvService) RejectRequest(ctx clidone.ServiceContext,
	args *provapi.RejectRequestArgs) error {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	slog.Info("RejectRequest",
		slog.String("senderID", ctx.SenderID),
		slog.String("deviceID", args.ClientID))
	status, found := svc.requests[args.ClientID]
	if !found {
		return fmt.Errorf("provisioning request for client '%s' not found", args.ClientID)
	}
	status.Pending = false
	status.RejectedMSE = time.Now().UnixMilli()
	svc.requests[args.ClientID] = status
	return nil
}

// SubmitRequest creates a provisioning request for a device
//
// If the request is pre-approved a token will be returned if the pubKey and/or
// MAC matches.
// If the pre-approval does not include a public key then only match required is the MAC.
func (svc *ManageIdProvService) SubmitRequest(ctx clidone.ServiceContext,
	args *provapi.ProvisionRequestArgs) (resp *provapi.ProvisionRequestResp, err error) {
	svc.mux.Lock()
	defer svc.mux.Unlock()
	var token string

	slog.Info("SubmitRequest",
		slog.String("senderID", ctx.SenderID),
		slog.String("deviceID", args.ClientID))
	status, found := svc.requests[args.ClientID]
	if !found {
		// new request
		status = provapi.ProvisionStatus{
			ClientID:    args.ClientID,
			PubKey:      args.PubKey,
			MAC:         args.MAC,
			Pending:     true,
			ReceivedMSE: time.Now().UnixMilli(),
			RetrySec:    60,
		}
	} else if status.ApprovedMSE != 0 {
		// (pre)approved request, add the user and issue a token
		status.ReceivedMSE = time.Now().UnixMilli()
		// public key or mac must match if provided
		if status.PubKey != "" && status.PubKey != args.PubKey {
			err = fmt.Errorf(
				"approval for '%s' denied as public key doesn't match", args.ClientID)
		} else if status.MAC != "" && status.MAC != args.MAC {
			err = fmt.Errorf(
				"approval for '%s' denied as mac address doesn't match", args.ClientID)
		} else if args.PubKey == "" {
			err = fmt.Errorf(
				"approval for '%s' denied as no public key was provided", args.ClientID)
		} else {
			err = nil
		}
		if err != nil {
			slog.Warn(err.Error(), "clientID", args.ClientID)
			return nil, err
		}

		status.Pending = false
		if status.ClientType == authapi.ClientTypeService {
			token, err = svc.authSvc.AddService(status.ClientID, "service", status.PubKey)
		} else {
			token, err = svc.authSvc.AddDevice(status.ClientID, "device", status.PubKey)
		}
		if err != nil {
			return nil, err
		}
		slog.Warn("provisioning token created for client", "clientID", args.ClientID)

	} else if status.RejectedMSE != 0 {
		// rejected request, ignore request
		// delay next request to 1 hour
		status.RetrySec = 3600
		status.ReceivedMSE = time.Now().UnixMilli()
		status.Pending = false
	} else {
		// repeat of request, update the request received timestamp and increase retry
		status.Pending = true
		status.ReceivedMSE = time.Now().UnixMilli()
		// delay next request to a maximum of 10 minutes
		if status.RetrySec < 600 {
			status.RetrySec += 30
		}
	}
	svc.requests[args.ClientID] = status
	resp = &provapi.ProvisionRequestResp{
		Status: status,
		Token:  token,
	}
	return resp, nil
}
func (svc *ManageIdProvService) Stop() {
}

func StartManageIdProvService(hc *clidone.HubClient) *ManageIdProvService {

	svc := &ManageIdProvService{
		// map of requests by SenderID
		requests: make(map[string]provapi.ProvisionStatus),
		hc:       hc,
	}

	// the auth service is used to create credentials
	svc.authSvc = authcli.NewManageClients(svc.hc)

	svc.hc.SetRPCCapability(provapi.ManageProvisioningCap,
		map[string]interface{}{
			provapi.ApproveRequestMethod:    svc.ApproveRequest,
			provapi.GetRequestsMethod:       svc.GetRequests,
			provapi.PreApproveClientsMethod: svc.PreApproveClients,
			provapi.RejectRequestMethod:     svc.RejectRequest,
			provapi.SubmitRequestMethod:     svc.SubmitRequest,
		})
	return svc
}
