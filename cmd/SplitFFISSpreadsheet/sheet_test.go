package main

import (
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
)

func TestParseXLSXFile_good(t *testing.T) {
	excelFixture, err := os.Open("fixtures/example_spreadsheet.xlsx")
	assert.NoError(t, err, "Error opening spreadsheet fixture")

	// Ignore logging in this test
	logger = log.NewNopLogger()

	opportunities, err := parseXLSXFile(excelFixture, logger)
	assert.NoError(t, err)
	assert.NotNil(t, opportunities)

	// Fixture has 4 opportunities
	assert.Len(t, opportunities, 4)

	// Our first row in the fixture sheet should match this exactly
	expectedOpportunity := ffis.FFISFundingOpportunity{
		CFDA:             "81.086",
		OppTitle:         "Example Opportunity 1",
		Agency:           "Office of Energy Efficiency and Renewable Energy",
		EstimatedFunding: 5000000,
		ExpectedAwards:   "N/A",
		OppNumber:        "ABC-0003065",
		GrantID:          123456,
		Eligibility: ffis.FFISFundingEligibility{
			State:           false,
			Local:           false,
			Tribal:          false,
			HigherEducation: false,
			NonProfits:      true,
			Other:           false,
		},
		Match: false,
		Bill:  "Infrastructure Investment and Jobs Act",
	}

	date, _ := time.Parse("1/2/2006", "5/11/2023")
	expectedOpportunity.DueDate = date

	assert.Equal(t, expectedOpportunity, opportunities[0])
}

func TestParseEligibility(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"X", true},
		{"N", false},
		{"", false},
		{"Invalid", false},
	}

	for _, tc := range testCases {
		actual := parseEligibility(tc.input)
		assert.Equal(t, tc.expected, actual)
	}
}
