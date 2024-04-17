package dirsrv

import (
	"encoding/json"
	"strings"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	dirapi "github.com/hiveot/hub/done_mod/mod_dir/dir_api"
	"github.com/hiveot/hub/done_tool/ser"

	"github.com/hiveot/hub/done_tool/things"
)

// convert the storage key and raw data to a things value object
// This returns the value, or nil if the key is invalid
func (svc *ReadDirectoryService) _decodeValue(key string, data []byte) (thingValue things.ThingValue, valid bool) {
	// key is constructed as  {timestamp}/{valueName}/{a|e}
	parts := strings.Split(key, "/")
	if len(parts) < 2 {
		return thingValue, false
	}
	thingValue = things.ThingValue{}
	_ = json.Unmarshal(data, &thingValue)
	return thingValue, true
}

// First returns the first entry in the directory
func (svc *ReadDirectoryService) First(
	ctx clidone.ServiceContext, args dirapi.CursorFirstArgs) (*dirapi.CursorFirstResp, error) {

	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}
	k, v, valid := cursor.First()
	if !valid {
		// store is empty
		return nil, nil
	}
	thingValue, valid := svc._decodeValue(k, v)
	resp := dirapi.CursorFirstResp{
		Value:     thingValue,
		Valid:     valid,
		CursorKey: args.CursorKey,
	}
	return &resp, nil
}

// Next moves the cursor to the next key from the current cursor
// First() or Seek must have been called first.
// Shouldn't next have a parameter?
func (svc *ReadDirectoryService) Next(
	ctx clidone.ServiceContext, args dirapi.CursorNextArgs) (*dirapi.CursorNextResp, error) {

	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}

	k, v, valid := cursor.Next()
	thingValue, valid := svc._decodeValue(k, v)
	resp := dirapi.CursorNextResp{
		Value:     thingValue,
		Valid:     valid,
		CursorKey: args.CursorKey,
	}
	return &resp, nil
}

// NextN moves the cursor to the next N places from the current cursor
// and return a list with N values in incremental time order.
// itemsRemaining is false if the iterator has reached the end.
// Intended to speed up with batch iterations over rpc.
func (svc *ReadDirectoryService) NextN(
	ctx clidone.ServiceContext, args dirapi.CursorNextNArgs) (*dirapi.CursorNextNResp, error) {

	cursor, err := svc.cursorCache.Get(args.CursorKey, ctx.SenderID, true)
	if err != nil {
		return nil, err
	}
	values := make([]things.ThingValue, 0, args.Limit)
	// obtain a map of [addr]TDJson
	docMap, itemsRemaining := cursor.NextN(args.Limit)
	for _, doc := range docMap {
		tv := things.ThingValue{}
		err2 := ser.Unmarshal(doc, &tv)
		if err2 == nil {
			values = append(values, tv)
		} else {
			err = err2 // return the last error
		}
	}
	resp := dirapi.CursorNextNResp{
		Values:         values,
		ItemsRemaining: itemsRemaining,
		CursorKey:      args.CursorKey,
	}
	return &resp, err
}

// Release close the cursor and release its resources.
// This invalidates all values obtained from the cursor
func (svc *ReadDirectoryService) Release(
	ctx clidone.ServiceContext, args dirapi.CursorReleaseArgs) error {

	return svc.cursorCache.Release(args.CursorKey, ctx.SenderID)
}
