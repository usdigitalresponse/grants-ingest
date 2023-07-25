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

func TestParseXLSXFile_cfda_format_4_digit(t *testing.T) {
	/*
		This test is for a spreadsheet that has a CFDA number with 4 digits
		instead of 3. This can happen if the CFDA ends in a 0 and Excel
		treats it as a number instead of a string.
	*/
	excelFixture, err := os.Open("fixtures/example_spreadsheet_four_digit_cfda.xlsx")
	assert.NoError(t, err, "Error opening spreadsheet fixture")

	// Ignore logging in this test
	logger = log.NewNopLogger()

	opportunities, err := parseXLSXFile(excelFixture, logger)
	assert.NoError(t, err)
	assert.NotNil(t, opportunities)

	// Fixture has 4 opportunities
	assert.Len(t, opportunities, 4)

	for idx, expectedRow := range []struct {
		expectedBill string
		expectedCFDA string
	}{
		{"Infrastructure Investment and Jobs Act", "81.086"},
		{"Inflation Reduction Act", "10.720"},
		{"Inflation Reduction Act", "81.253"},
		{"Department of Agriculture", "02.980"},
	} {
		assert.Equal(t, expectedRow.expectedBill, opportunities[idx].Bill)
		assert.Equal(t, expectedRow.expectedCFDA, opportunities[idx].CFDA)
	}
}
