package dirsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	vocab "github.com/hiveot/hub/done_api/api_go"
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	dirapi "github.com/hiveot/hub/done_mod/mod_dir/dir_api"
	"github.com/hiveot/hub/done_tool/buckets"
	"github.com/hiveot/hub/done_tool/things"
)

// ReadDirectoryService is a provides the capability to read and iterate the directory
type ReadDirectoryService struct {
	// read bucket that holds the TD documents
	bucket buckets.IBucket
	// cache of remote cursors
	cursorCache *buckets.CursorCache
}

// CreateReadDirTD creates a Thing TD document describing the read directory capability
func (svc *ReadDirectoryService) CreateReadDirTD() *things.TD {
	title := "Thing Directory Reader"
	deviceType := vocab.ThingServiceDirectory
	td := things.NewTD(dirapi.ReadDirectoryCap, title, deviceType)
	// TODO: add properties
	return td
}

// GetCursor returns an iterator for ThingValues containing a TD document
// The lifespan is currently fixed to 1 minute.
//
//	clientID is the owner of the cursor. Used to remove all cursors of an owner when it disconnects.
func (svc *ReadDirectoryService) GetCursor(
	ctx clidone.ServiceContext) (*dirapi.GetCursorResp, error) {

	dirCursor, err := svc.bucket.Cursor(context.Background())
	if err == nil {
		// TODO: what lifespan is reasonable?
		key := svc.cursorCache.Add(dirCursor, svc.bucket, ctx.SenderID, time.Minute)
		resp := &dirapi.GetCursorResp{CursorKey: key}
		return resp, nil
	}
	return nil, err
}

// GetTD returns the TD document for the given Thing ID in JSON format
func (svc *ReadDirectoryService) GetTD(
	ctx clidone.ServiceContext, args *dirapi.GetTDArgs) (resp *dirapi.GetTDResp, err error) {

	//logrus.Infof("agentID=%s, thingID=%s", svc.agentID, thingID)
	// store keys are made of the agentID / thingID
	thingAddr := args.AgentID + "/" + args.ThingID
	raw, err := svc.bucket.Get(thingAddr)
	resp = &dirapi.GetTDResp{}
	if raw != nil {
		tv := things.ThingValue{}
		err = json.Unmarshal(raw, &tv)
		resp.Value = tv
	} else {
		err = fmt.Errorf("TD with agentID '%s' and thingID '%s' not found ", args.AgentID, args.ThingID)
	}
	return resp, err
}

// GetTDsRaw returns a collection of ThingValue documents
// Intended for transferring documents without unnecessary marshalling
func (svc *ReadDirectoryService) GetTDsRaw(
	ctx clidone.ServiceContext, args *dirapi.GetTDsArgs) (map[string][]byte, error) {

	cursor, err := svc.bucket.Cursor(context.Background())
	if args.Offset > 0 {
		// TODO: add support for cursor.Skip
		cursor.NextN(uint(args.Offset))
	}

	docs, itemsRemaining := cursor.NextN(uint(args.Limit))
	_ = itemsRemaining
	return docs, err
}

// GetTDs returns a collection of TD documents
// this is rather inefficient. Should the client do the unmarshalling of the docs array?
// that would break the matching API. Maybe an internal method that returns a raw batch?
func (svc *ReadDirectoryService) GetTDs(
	ctx clidone.ServiceContext, args *dirapi.GetTDsArgs) (res *dirapi.GetTDsResp, err error) {

	batch := make([]things.ThingValue, 0, args.Limit)
	cursor, err := svc.bucket.Cursor(context.Background())
	if args.Offset > 0 {
		// FIXME: add support for cursor.Skip
		cursor.NextN(uint(args.Offset))
	}
	docs, itemsRemaining := cursor.NextN(uint(args.Limit))
	// FIXME: the unmarshalled ThingValue will be remarshalled when sending it as a reply.
	_ = itemsRemaining
	for key, val := range docs {
		tv := things.ThingValue{}
		err = json.Unmarshal(val, &tv)
		if err == nil {
			batch = append(batch, tv)
		} else {
			slog.Warn("unable to unmarshal TV", "err", err, "key", key)
		}
	}
	res = &dirapi.GetTDsResp{Values: batch}
	return res, err
}

//// ListTDs returns an array of TD documents in JSON text
//func (srv *DirectoryKVStoreServer) ListTDs(_ context.Context, limit int, offset int) ([]string, error) {
//	res := make([]string, 0)
//	docs, err := srv.store.List(srv.defaultBucket, limit, offset, nil)
//	if err == nil {
//		for _, doc := range docs {
//			res = append(res, doc)
//		}
//	}
//	return res, err
//}

// ListTDcb provides a callback with an array of TD documents in JSON text
//func (srv *DirectoryKVStoreServer) ListTDcb(
//	ctx context.Context, handler func(td string, isLast bool) error) error {
//	_ = ctx
//	batch := make([]string, 0)
//	docs, err := srv.store.List(srv.defaultBucket, 0, 0, nil)
//	if err == nil {
//		// convert map to array
//		for _, doc := range docs {
//			batch = append(batch, doc)
//		}
//		// for testing, callback one at a time
//		//err = handler(batch, true)
//		for i, tddoc := range batch {
//			docList := []string{tddoc}
//			isLast := i == len(batch)-1
//			err = handler(docList, isLast)
//		}
//	}
//	return err
//}

// QueryTDs returns an array of TD documents that match the jsonPath query
//  thingIDs optionally restricts the result to the given IDs
//func (srv *DirectoryKVStoreServer) QueryTDs(_ context.Context, jsonPathQuery string, limit int, offset int) ([]string, error) {
//
//	resp, err := srv.store.Query(jsonPathQuery, limit, offset, nil)
//	return resp, err
//	//res := make([]string, 0)
//	//if err == nil {
//	//	for _, docText := range resp {
//	//		var td things.ThingDescription
//	//		err = json.Unmarshal([]byte(docText), &td)
//	//		res.Things = append(res.Things, &td)
//	//	}
//	//}
//	//return res, err
//}

// QueryTDs returns the TD's filtered using JSONpath on the TD content
// See 'docs/query-tds.md' for examples
// disabled as this is not used
//QueryTDs(ctx context.Context, jsonPath string, limit int, offset int) (tds []string, err error)

// Stop the read directory capability
// this unsubscribes from requests and stops the cursor cleanup task.
func (svc *ReadDirectoryService) Stop() {
	svc.cursorCache.Stop()
}

// StartReadDirectoryService starts the capability to read the directory
// hc with the message bus connection. Its ID will be used as the agentID that provides the capability.
// bucket is an open store bucket for reading the TD data.
func StartReadDirectoryService(hc *clidone.HubClient, bucket buckets.IBucket) *ReadDirectoryService {

	svc := &ReadDirectoryService{
		bucket:      bucket,
		cursorCache: buckets.NewCursorCache(),
	}
	capMethods := map[string]interface{}{
		dirapi.CursorFirstMethod:   svc.First,
		dirapi.CursorNextMethod:    svc.Next,
		dirapi.CursorNextNMethod:   svc.NextN,
		dirapi.CursorReleaseMethod: svc.Release,
		dirapi.GetCursorMethod:     svc.GetCursor,
		dirapi.GetTDMethod:         svc.GetTD,
		dirapi.GetTDsMethod:        svc.GetTDs,
	}
	// listen for requests
	hc.SetRPCCapability(dirapi.ReadDirectoryCap, capMethods)

	svc.cursorCache.Start()
	return svc
}
