package purgeData

import (
	"github.com/usdigitalresponse/grants-ingest/cli/grants-ingest/purgeData/preparedDataBucket"
)

type Cmd struct {
	// Sub-commands
	PreparedDataBucket preparedDataBucket.Cmd `cmd:"prepared-data-bucket" help:"Purge data from the prepared data S3 bucket."`
}
