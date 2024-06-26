package histsrv

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	"github.com/hiveot/hub/done_cli/cli_done/transport"
	histapi "github.com/hiveot/hub/done_mod/mod_hist/hist_api"
	"github.com/hiveot/hub/done_tool/buckets"

	"github.com/hiveot/hub/done_tool/things"
)

// key of filter by event/action name, stored in context
const filterContextKey = "name"

// The HistoryCursor contains the bucket instance created for a cursor.
// It is created when a cursor is requested, stored in the cursorCache and
// released when the cursor is released or expires.
//type HistoryCursor struct {
//	// agentID
//	agentID    string //
//	thingID    string
//	filterName string                // optional event name to filter on
//	bucket     buckets.IBucket       // bucket being iterator
//	bc         buckets.IBucketCursor // the iteration
//}

// convert the storage key and raw data to a things value object
// this must match the encoding done in AddHistory
//
//	bucketID is the ID of the bucket, which this service defines as agentID/thingID
//	key is the value's key, which is defined as timestamp/valueName
//
// This returns the value, or nil if the key is invalid
func decodeValue(bucketID string, key string, data []byte) (thingValue *things.ThingValue, valid bool) {

	// key is constructed as  timestamp/name/{a|e|c}/sender, where sender can be omitted
	parts := strings.Split(key, "/")
	if len(parts) < 2 {
		return thingValue, false
	}
	millisec, _ := strconv.ParseInt(parts[0], 10, 64)
	name := parts[1]
	senderID := ""
	messageType := transport.MessageTypeEvent
	if len(parts) >= 2 {
		if parts[2] == "a" {
			messageType = transport.MessageTypeAction
		} else if parts[2] == "c" {
			messageType = transport.MessageTypeConfig
		}
	}
	if len(parts) > 3 {
		senderID = parts[3]
	}

	// the bucketID consists of the agentID/thingID
	addrParts := strings.Split(bucketID, "/")
	if len(addrParts) < 2 {
		return nil, false
	}
	agentID := addrParts[0]
	thingID := addrParts[1]

	thingValue = &things.ThingValue{
		ThingID:     thingID,
		AgentID:     agentID,
		Name:        name,
		Data:        data,
		CreatedMSec: millisec,
		ValueType:   messageType,
		SenderID:    senderID,
	}
	return thingValue, true
}

// findNextName iterates the cursor until the next value containing 'name' is found and the
// timestamp doesn't exceed untilTime.
// A successive call with an increased timestamp should return the next batch of results. Intended
// to iterated an hours/day/week at a time.
// This returns the next value, or nil if the value was not found.
//
//	cursor to iterate
//	name is the event name to match
//	until is the time not to exceed in the result. Intended to avoid unnecessary iteration in range queries
func (svc *ReadHistoryService) findNextName(
	cursor buckets.IBucketCursor, name string, until time.Time) (thingValue *things.ThingValue, found bool) {
	found = false
	for {
		k, v, valid := cursor.Next()
		if !valid {
			// key is invalid. This means we reached the end of cursor
			return nil, false
		}
		// key is constructed as  {timestamp}/{valueName}/{a|e|c}/{sender}
		parts := strings.Split(k, "/")
		if len(parts) != 4 {
			// key exists but is invalid. skip this entry
			slog.Warn("findNextName: invalid key", "key", k)
		} else {
			// check timestamp and name must match
			timestampmsec, _ := strconv.ParseInt(parts[0], 10, 64)
			if timestampmsec > until.UnixMilli() {
				// we passed the given time limit
				// undo the last step so that followup requests with a new time limit can include this result
				cursor.Prev()
				return thingValue, false
			}
			if name == parts[1] {
				// found a match. Decode and return it
				thingValue, found = decodeValue(cursor.BucketID(), k, v)
				return
			}
			// name doesn't match. Skip this entry
		}
	}
}

// findPrevName iterates the cursor until the previous value containing 'name' is found and the
// timestamp is not before 'until' time.
// A successive call with an increased timestamp should return the next batch of results. Intended
// to iterate an hours/day/week at a time.
// This returns the previous value, or nil if the value was not found.
//
//	name is the event name to match
//	until is the time not to exceed in the result. Intended to avoid unnecesary iteration in range queries
func (svc *ReadHistoryService) findPrevName(
	cursor buckets.IBucketCursor, name string, until time.Time) (thingValue *things.ThingValue, found bool) {
	found = false
	for {
		k, v, valid := cursor.Prev()
		if !valid {
			// key is invalid. This means we reached the beginning of cursor
			return thingValue, false
		}
		// key is constructed as  {timestamp}/{valueName}/{a|e|c}/sender
		parts := strings.Split(k, "/")
		if len(parts) != 4 {
			// key exists but is invalid. skip this entry
		} else {
			// check timestamp and name must match
			timestampmsec, _ := strconv.ParseInt(parts[0], 10, 64)
			if timestampmsec < until.UnixMilli() {
				// we passed the given time limit
				// undo the last step so that followup requests with a new time limit can include this result
				cursor.Next()
				return thingValue, false
			}
			if name == parts[1] {
				// found a match. Decode and return it
				thingValue, found = decodeValue(cursor.BucketID(), k, v)
				return
			}
			// name doesn't match. Skip this entry
		}
	}
}

// First returns the oldest value in the history
func (svc *ReadHistoryService) First(
	ctx clidone.ServiceContext, args histapi.CursorArgs) (*histapi.CursorSingleResp, error) {
	until := time.Now()

	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}
	k, v, valid := cursor.First()
	if !valid {
		// bucket is empty
		return nil, nil
	}

	thingValue, valid := decodeValue(cursor.BucketID(), k, v)
	filterName := cursor.Context().Value(filterContextKey).(string)
	if valid && filterName != "" && thingValue.Name != filterName {
		thingValue, valid = svc.findNextName(cursor, filterName, until)
	}
	resp := histapi.CursorSingleResp{
		Value: thingValue,
		Valid: valid,
	}
	return &resp, err
}

// Last positions the cursor at the last key in the ordered list
func (svc *ReadHistoryService) Last(
	ctx clidone.ServiceContext, args histapi.CursorArgs) (*histapi.CursorSingleResp, error) {

	// the beginning of time?
	until := time.Time{}
	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}
	k, v, valid := cursor.Last()

	resp := &histapi.CursorSingleResp{
		Value: nil,
		Valid: valid,
	}

	if !valid {
		// bucket is empty
		return resp, nil
	}
	thingValue, valid := decodeValue(cursor.BucketID(), k, v)
	filterName := cursor.Context().Value(filterContextKey).(string)
	if valid && filterName != "" && thingValue.Name != filterName {
		thingValue, valid = svc.findPrevName(cursor, filterName, until)
	}
	resp.Value = thingValue
	resp.Valid = valid
	return resp, nil
}

// Next moves the cursor to the next key from the current cursor
// First() or Seek must have been called first.
func (svc *ReadHistoryService) Next(
	ctx clidone.ServiceContext, args histapi.CursorArgs) (*histapi.CursorSingleResp, error) {

	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}
	k, v, valid := cursor.Next()
	resp := &histapi.CursorSingleResp{
		Value: nil,
		Valid: valid,
	}
	if !valid {
		return resp, nil
	}
	thingValue, valid := decodeValue(cursor.BucketID(), k, v)
	filterName := cursor.Context().Value(filterContextKey).(string)
	if valid && filterName != "" && filterName != thingValue.Name {
		until := time.Now()
		thingValue, valid = svc.findNextName(cursor, filterName, until)
	}

	resp.Value = thingValue
	resp.Valid = valid

	return resp, nil
}

// NextN moves the cursor to the next N places from the current cursor
// and return a list with N values in incremental time order.
// itemsRemaining is false if the iterator has reached the end.
// Intended to speed up with batch iterations over rpc.
func (svc *ReadHistoryService) NextN(
	ctx clidone.ServiceContext, args histapi.CursorNArgs) (*histapi.CursorNResp, error) {

	values := make([]*things.ThingValue, 0, args.Limit)
	nextArgs := histapi.CursorArgs{CursorKey: args.CursorKey}
	itemsRemaining := true

	// tbd is it faster to use NextN and sort the keys?
	for i := 0; i < args.Limit; i++ {
		nextResp, err := svc.Next(ctx, nextArgs)
		if !nextResp.Valid || err != nil {
			itemsRemaining = false
			break
		}
		values = append(values, nextResp.Value)
	}
	resp := &histapi.CursorNResp{}
	resp.Values = values
	resp.ItemsRemaining = itemsRemaining
	return resp, nil
}

// Prev moves the cursor to the previous key from the current cursor
// Last() or Seek must have been called first.
func (svc *ReadHistoryService) Prev(
	ctx clidone.ServiceContext, args histapi.CursorArgs) (*histapi.CursorSingleResp, error) {

	until := time.Time{}
	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}
	k, v, valid := cursor.Prev()
	resp := &histapi.CursorSingleResp{
		Value: nil,
		Valid: valid,
	}
	if !valid {
		return resp, nil
	}
	thingValue, valid := decodeValue(cursor.BucketID(), k, v)
	filterName := cursor.Context().Value(filterContextKey).(string)
	if valid && filterName != "" && filterName != thingValue.Name {
		thingValue, valid = svc.findPrevName(cursor, filterName, until)
	}
	resp.Value = thingValue
	resp.Valid = valid
	return resp, nil
}

// PrevN moves the cursor back N places from the current cursor
// and return a list with N values in decremental time order.
// itemsRemaining is true if the iterator has reached the beginning
// Intended to speed up with batch iterations over rpc.
func (svc *ReadHistoryService) PrevN(
	ctx clidone.ServiceContext, args histapi.CursorNArgs) (*histapi.CursorNResp, error) {

	values := make([]*things.ThingValue, 0, args.Limit)
	prevArgs := histapi.CursorArgs{CursorKey: args.CursorKey}
	itemsRemaining := true

	// tbd is it faster to use NextN and sort the keys? - for a remote store yes
	for i := 0; i < args.Limit; i++ {
		prevResp, err := svc.Prev(ctx, prevArgs)
		if !prevResp.Valid || err != nil {
			itemsRemaining = false
			break
		}
		values = append(values, prevResp.Value)
	}
	resp := &histapi.CursorNResp{}
	resp.Values = values
	resp.ItemsRemaining = itemsRemaining
	return resp, nil
}

// Release closes the bucket and cursor
// This invalidates all values obtained from the cursor
func (svc *ReadHistoryService) Release(
	ctx clidone.ServiceContext, args histapi.CursorReleaseArgs) error {

	return svc.cursorCache.Release(ctx.SenderID, args.CursorKey)
}

// Seek positions the cursor at the given searchKey and corresponding value.
// If the key is not found, the next key is returned.
func (svc *ReadHistoryService) Seek(
	ctx clidone.ServiceContext, args histapi.CursorSeekArgs) (*histapi.CursorSingleResp, error) {

	until := time.Now()
	//ts, err := dateparse.ParseAny(timestampMsec)
	//if err != nil {
	slog.Info("Seek using timestamp",
		slog.Int64("timestampMsec", args.TimeStampMSec),
		slog.String("clientID", ctx.SenderID),
	)

	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}

	// search the first occurrence at or after the given timestamp
	// the buck index uses the stringified timestamp
	searchKey := strconv.FormatInt(args.TimeStampMSec, 10) //+ "/" + thingValue.ID

	k, v, valid := cursor.Seek(searchKey)
	resp := &histapi.CursorSingleResp{
		Value: nil,
		Valid: valid,
	}
	if !valid {
		// bucket is empty
		return resp, nil
	}
	thingValue, valid := decodeValue(cursor.BucketID(), k, v)
	filterName := cursor.Context().Value(filterContextKey).(string)
	if valid && filterName != "" && thingValue.Name != filterName {
		thingValue, valid = svc.findNextName(cursor, filterName, until)
	}
	resp.Value = thingValue
	resp.Valid = valid

	return resp, nil
}
