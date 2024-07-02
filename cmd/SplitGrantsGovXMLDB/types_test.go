package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpportunityS3ObjectKey(t *testing.T) {
	opp := &opportunity{OpportunityID: "123456789"}
	assert.Equal(t, opp.s3ObjectKey(), "123/123456789/grants.gov/v2.OpportunitySynopsisDetail_1_0.xml")
}

func TestForecastS3ObjectKey(t *testing.T) {
	f := &forecast{OpportunityID: "123456789"}
	assert.Equal(t, f.s3ObjectKey(), "123/123456789/grants.gov/v2.OpportunityForecastDetail_1_0.xml")
}
