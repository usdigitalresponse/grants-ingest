package purgeData

import (
	"github.com/usdigitalresponse/grants-ingest/cli/grants-ingest/purgeData/preparedDataBucket"
	"github.com/usdigitalresponse/grants-ingest/cli/grants-ingest/purgeData/preparedDataTable"
)

type Cmd struct {
	// Sub-commands
	PreparedDataBucket preparedDataBucket.Cmd `cmd:"prepared-data-bucket" help:"Purge data from the prepared data S3 bucket."`
	PreparedDataTable  preparedDataTable.Cmd  `cmd:"prepared-data-table" help:"Purge data from the prepared data DynamoDB table."`
}
