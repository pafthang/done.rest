package dirsrv

import (
	"encoding/json"
	"log/slog"

	vocab "github.com/hiveot/hub/done_api/api_go"
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	"github.com/hiveot/hub/done_cli/cli_done/transport"
	dirapi "github.com/hiveot/hub/done_mod/mod_dir/dir_api"
	"github.com/hiveot/hub/done_tool/buckets"
	"github.com/hiveot/hub/done_tool/things"
)

// UpdateDirectoryService is a provides the capability to update the directory
// This implements the IUpdateDirectory API
//
//	Bucket keys are made of gatewayID+"/"+thingID
//	Bucket values are ThingValue objects
type UpdateDirectoryService struct {
	// bucket that holds the TD documents
	bucket buckets.IBucket
}

// CreateUpdateDirTD a new Thing TD document describing the update directory capability
func (svc *UpdateDirectoryService) CreateUpdateDirTD() *things.TD {
	title := "Thing Directory Updater"
	deviceType := vocab.ThingServiceDirectory
	td := things.NewTD(dirapi.UpdateDirectoryCap, title, deviceType)
	// TODO: add properties
	return td
}

func (svc *UpdateDirectoryService) RemoveTD(ctx clidone.ServiceContext, args dirapi.RemoveTDArgs) error {
	slog.Info("RemoveTD",
		slog.String("senderID", ctx.SenderID),
		slog.String("agentID", args.AgentID),
		slog.String("thingID", args.ThingID))

	thingAddr := args.AgentID + "/" + args.ThingID
	err := svc.bucket.Delete(thingAddr)
	return err
}

func (svc *UpdateDirectoryService) UpdateTD(ctx clidone.ServiceContext, args dirapi.UpdateTDArgs) error {
	slog.Info("UpdateTD",
		slog.String("senderID", ctx.SenderID),
		slog.String("agentID", args.AgentID),
		slog.String("thingID", args.ThingID))

	// store the TD ThingValue
	thingValue := things.NewThingValue(
		transport.MessageTypeEvent, args.AgentID, args.ThingID, transport.EventNameTD, args.TDDoc, ctx.SenderID)
	bucketData, _ := json.Marshal(thingValue)
	thingAddr := args.AgentID + "/" + args.ThingID
	err := svc.bucket.Set(thingAddr, bucketData)
	return err
}

// Stop the update directory capability
// This unsubscribes from requests.
func (svc *UpdateDirectoryService) Stop() {
}

// StartUpdateDirectoryService starts the capability to update the directory.
// Invoke Stop() when done to unsubscribe from requests.
//
//	hc with the message bus connection
//	thingBucket is the open bucket used to store TDs
func StartUpdateDirectoryService(hc *clidone.HubClient, bucket buckets.IBucket) *UpdateDirectoryService {

	svc := &UpdateDirectoryService{
		bucket: bucket,
	}
	capMethods := map[string]interface{}{
		dirapi.UpdateTDMethod: svc.UpdateTD,
		dirapi.RemoveTDMethod: svc.RemoveTD,
	}
	hc.SetRPCCapability(dirapi.UpdateDirectoryCap, capMethods)

	return svc
}
