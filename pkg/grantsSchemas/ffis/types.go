package ffis

import "time"

// message payload for FFIS download operations
type FFISMessageDownload struct {
	SourceFileKey string `json:"sourceFileKey"`
	DownloadURL   string `json:"downloadUrl"`
}

// Represents a funding opportunity sourced from an FFIS spreadsheet
type FFISFundingOpportunity struct {
	CFDA     string `json:"cfda"`              // eg. 11.525
	OppTitle string `json:"opportunity_title"` // eg. "FY 2020 Community Connect Grant Program"

	// In the FFIS spreadsheet, this is the header row for a group of opportunities
	OppCategory string `json:"opportunity_category"` // eg. Inflation Reduction Act

	Agency           string                 `json:"opportunity_agency"` // eg. Forest Service
	EstimatedFunding int64                  `json:"estimated_funding"`  // eg. $25,000,000
	ExpectedAwards   string                 `json:"expected_awards"`    // eg. 10 or N/A
	OppNumber        string                 `json:"opportunity_number"` // eg. USDA-FS-2020-01
	GrantID          int64                  `json:"grant_id"`           // eg. 347509
	Eligibility      FFISFundingEligibility `json:"eligibility"`
	DueDate          time.Time              `json:"due_date"`

	Match bool `json:"match"`
}

// Elegibility for FFIS funding opportunities as presented in FFIS spreadsheets
type FFISFundingEligibility struct {
	State           bool `json:"state"`            // State Governments
	Local           bool `json:"local"`            // Local Governments
	Tribal          bool `json:"tribal"`           // Tribal Governments
	HigherEducation bool `json:"higher_education"` // Institutions of Higher Education
	NonProfits      bool `json:"non_profits"`      // Non-profits
	Other           bool `json:"other"`            // Other/see announcement
}
