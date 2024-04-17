package boltstore

import (
	"path"

	"github.com/hiveot/hub/done_tool/buckets"
	"github.com/hiveot/hub/done_tool/buckets/bolts"
)

// NewBucketStore creates a new bucket store of a given type
// The store will be created in the given directory using the
// backend as the name. The directory is typically the name of the service that
// uses the store. Different databases can co-exist.
//
//	directory is the directory in which to create the store
//	name of the store database file or folder without extension
//	backend is the type of store to create: BackendKVBTree, BackendBBolt, BackendPebble
func NewBoltsStore(directory, name string, backend string) (store buckets.IBucketStore) {

	// bbolt stores data into a single file
	storePath := path.Join(directory, name+".boltdb")
	store = bolts.NewBoltStore(storePath)

	return store
}
