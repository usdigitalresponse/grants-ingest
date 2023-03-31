package grantsgov_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
)

// See https://stackoverflow.com/a/43497333
func randomDate(t *testing.T) time.Time {
	t.Helper()
	min := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	max := time.Date(2070, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	sec := rand.Int63n(max-min) + min
	return time.Unix(sec, 0)
}

func TestMMDDYYYYTypeTime(t *testing.T) {
	for i := 0; i < 100; i++ {
		expected := randomDate(t)
		t.Run(fmt.Sprintf("From date %s", expected), func(t *testing.T) {
			v := grantsgov.MMDDYYYYType(expected.Format(grantsgov.TimeLayoutMMDDYYYYType))
			actual, err := v.Time()
			assert.NoError(t, err)
			assert.Equalf(t, expected.Year(), actual.Year(),
				"Year does not match in MMDDYYYType.Time() %s", actual)
			assert.Equalf(t, expected.Month(), actual.Month(),
				"Month does not match in MMDDYYYType.Time() %s", actual)
			assert.Equalf(t, expected.Day(), actual.Day(),
				"Day does not match in MMDDYYYType.Time() %s", actual)
		})
	}
}

func TestFiscaleYearTypeTime(t *testing.T) {
	for year := 1970; year <= 2070; year++ {
		expected := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		t.Run(fmt.Sprintf("From year %d", year), func(t *testing.T) {
			v := grantsgov.FiscalYearType(expected.Format(grantsgov.TimeLayoutFiscalYearType))
			actual, err := v.Time()
			assert.NoError(t, err)
			assert.Equal(t, expected.Year(), actual.Year(),
				"Year does not match in FiscalYearType.Time() %s", actual)
		})
	}
}
