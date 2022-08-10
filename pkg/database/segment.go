package database

import (
	"github.com/newrelic/go-agent/v3/newrelic"
)

// startDatastoreSegment configures and starts a datastore segment
func (d *Db) startDatastoreSegment(txn *newrelic.Transaction, operation, tableName string) *newrelic.DatastoreSegment {
	if txn == nil {
		d.Logger.Panic("nil transaction")
	}

	return &newrelic.DatastoreSegment{
		Product:    newrelic.DatastorePostgres,
		Collection: tableName,
		Operation:  operation,
		StartTime:  txn.StartSegmentNow(),
	}
}
