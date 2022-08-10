package database

import (
	"context"

	schema "github.com/SimifiniiCTO/simfiny-microservice-template/pkg/gen/proto/service_schema"
	"github.com/newrelic/go-agent/v3/newrelic"
	core_database "github.com/yoanyombapro1234/FeelGuuds_Core/core/core-database"
)

func (d *Db) CreateAccount(ctx context.Context, acct *schema.UserAccount) (*schema.UserAccount, error) {
	return nil, nil
}

func (d *Db) createAccountTxn(ctx context.Context, txn *newrelic.Transaction, acct *schema.UserAccount) core_database.CmplxTx {
	return nil
}

func (d *Db) GetAccount(ctx context.Context, acctID uint64) (*schema.UserAccount, error) {
	return nil, nil
}

func (d *Db) getAccountTxn(ctx context.Context, txn *newrelic.Transaction, acctID uint64) core_database.CmplxTx {
	return nil
}
