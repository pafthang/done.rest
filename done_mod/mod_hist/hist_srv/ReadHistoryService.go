package histsrv

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	histapi "github.com/hiveot/hub/done_mod/mod_hist/hist_api"
	"github.com/hiveot/hub/done_tool/buckets"

	"github.com/hiveot/hub/done_tool/things"
)

// GetPropertiesFunc is a callback function to retrieve latest properties of a Thing
// latest properties are stored separate from the history.
type GetPropertiesFunc func(thingAddr string, names []string) things.ThingValueMap

// ReadHistoryService provides read access to the history of things values.
type ReadHistoryService struct {
	// routing address of the things to read history of
	bucketStore buckets.IBucketStore
	// cache of remote cursors
	cursorCache *buckets.CursorCache
	// The service implements the getPropertyValues function as it does the caching and
	// provides concurrency control.
	getPropertiesFunc GetPropertiesFunc

	isRunning bool
}

// GetCursor returns an iterator for ThingValues containing a TD document
// The inactivity lifespan is currently fixed to 1 minute.
func (svc *ReadHistoryService) GetCursor(
	ctx clidone.ServiceContext, args histapi.GetCursorArgs) (*histapi.GetCursorResp, error) {

	if args.AgentID == "" || args.ThingID == "" {
		return nil, fmt.Errorf("missing agentID or thingID from client '%s'", ctx.SenderID)
	}
	thingAddr := args.AgentID + "/" + args.ThingID
	slog.Debug("GetCursor for bucket: ", "addr", thingAddr)
	bucket := svc.bucketStore.GetBucket(thingAddr)
	bctx := context.WithValue(context.Background(), filterContextKey, args.Name)
	cursor, err := bucket.Cursor(bctx)
	if err != nil {
		return nil, err
	}
	key := svc.cursorCache.Add(cursor, bucket, ctx.SenderID, time.Minute)
	resp := &histapi.GetCursorResp{CursorKey: key}
	return resp, nil
}

// GetLatest returns the most recent property and event values of the Thing.
// Latest Properties are tracked in a 'latest' record which holds a map of propertyName:ThingValue records
//
//	providing 'names' can speed up read access significantly
func (svc *ReadHistoryService) GetLatest(
	ctx clidone.ServiceContext, args *histapi.GetLatestArgs) (*histapi.GetLatestResp, error) {
	thingAddr := args.AgentID + "/" + args.ThingID
	values := svc.getPropertiesFunc(thingAddr, args.Names)
	resp := histapi.GetLatestResp{Values: values}
	return &resp, nil
}

// Stop the read history capability
// this unsubscribes from requests and stops the cursor cleanup task.
func (svc *ReadHistoryService) Stop() {
	svc.isRunning = false
	svc.cursorCache.Stop()
}

// StartReadHistoryService starts the capability to read from a things's history
//
//	hc with the message bus connection. Its ID will be used as the agentID that provides the capability.
//	thingBucket is the open bucket used to store history data
//	getPropertiesFunc implements the aggregation of the Thing's most recent property values
func StartReadHistoryService(
	hc *clidone.HubClient, bucketStore buckets.IBucketStore, getPropertiesFunc GetPropertiesFunc,
) (svc *ReadHistoryService, err error) {

	svc = &ReadHistoryService{
		bucketStore:       bucketStore,
		getPropertiesFunc: getPropertiesFunc,
		cursorCache:       buckets.NewCursorCache(),
	}
	svc.cursorCache.Start()
	capMethods := map[string]interface{}{
		histapi.CursorFirstMethod:   svc.First,
		histapi.CursorLastMethod:    svc.Last,
		histapi.CursorNextMethod:    svc.Next,
		histapi.CursorNextNMethod:   svc.NextN,
		histapi.CursorPrevMethod:    svc.Prev,
		histapi.CursorPrevNMethod:   svc.PrevN,
		histapi.CursorReleaseMethod: svc.Release,
		histapi.CursorSeekMethod:    svc.Seek,
		histapi.GetCursorMethod:     svc.GetCursor,
		histapi.GetLatestMethod:     svc.GetLatest,
	}
	hc.SetRPCCapability(histapi.ReadHistoryCap, capMethods)
	return svc, err
}
