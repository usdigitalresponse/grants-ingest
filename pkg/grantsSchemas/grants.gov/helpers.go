//go:generate sh generate.sh
package grantsgov

import (
	"time"
)

const TimeLayoutMMDDYYYYType = "01022006"

func (v MMDDYYYYType) Time() (time.Time, error) {
	return time.Parse(TimeLayoutMMDDYYYYType, string(v))
}

const TimeLayoutFiscalYearType = "2006"

func (v FiscalYearType) Time() (time.Time, error) {
	return time.Parse(TimeLayoutFiscalYearType, string(v))
}

type OpportunitySynopsisDetail_1_0 OpportunitySynopsisDetail10
type OpportunityForecastDetail_1_0 OpportunityForecastDetail10
