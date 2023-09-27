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

func (cmd *Cmd) Help() string {
	return `
This command serves as the entrypoint for subcommands that purge specific types of data from
an environment, especially when targeting data from a specific source (e.g. Grants.gov or FFIS.org),
presumably before running a backfill operation that restores the purged data. These operations
may prove useful in scenarios following a bug that resulted in corrupted data, or where downstream
consumers of published events require new events to be published (which should be avoided in favor
of alternatives, especially if the number of such consumers grows over time, in order to keep the
event-publishing behaviors of this service consistent with its documentation).

In most cases, purge operations that target Grants.gov data are the easiest and most effective
to orchestrate.
The following serves as a runbook for these scenarios:
1. Disable any DynamoDB-based streams/triggers. Although not strictly necessary, this step
	may be useful in scenarios where quick restoration is warranted, and helps keep data which
	is known to be corrupt from entering the stream in the first place.
2. Purge Grants.gov data from S3 by running the prepared-data-bucket subcommand with --purge-gov
	option.
3. Purge Grants.gov data from DynamoDB by running the prepared-data-table subcommand with the
	--purge-gov option.
4. Restore DynamoDB-based streams/triggers disabled in Step 1.
5. Trigger Lambda execution to re-ingest the purged data (or wait for the Lambda execution
	to run according to schedule).

In addition to the workflow enumerated above, subcommands may be executed on an as-needed basis.
Given the destructive nature of these operations, please exercise caution, especially when dealing
with Production and other shared environments' data.

Observe the following recommended best practices:
- Always test workflows involving these commands against lower environments.
- Make use of --dry-run and --log-level=debug options when running against sensitive environments.
- Consider making bash scripts that can be peer-reviewed to guard against human error.
- Make backups of important data before performing destructive actions.
- Always communicate explicitly before running these commands against sensitive and/or shared
	environments.

Finally, it is worth noting that these operations are optimized for disaster-recovery and other
time-sensitive scenarios rather than cost. Testing is of course encouraged, but users should
be aware that scan-type operations like these can be costly when run against large data sets.
Therefore, avoid running these commands in a scheduled or automated fashion that can potentially
result in an unbounded number of executions.`
}
