package ffis

import "time"

// FFISMessageDownload is the message payload for FFIS download operations
type FFISMessageDownload struct {
	SourceFileKey string `json:"sourceFileKey"`
	DownloadURL   string `json:"downloadUrl"`
}

// Represents a funding opportunity sourced from an FFIS spreadsheet
type FFISFundingOpportunity struct {
	Agency           string                 `json:"opportunity_agency"` // eg. Forest Service
	Bill             string                 `json:"bill"`               // eg. Inflation Reduction Act
	CFDA             string                 `json:"cfda"`               // eg. 11.525
	DueDate          time.Time              `json:"due_date"`
	Eligibility      FFISFundingEligibility `json:"eligibility"`
	EstimatedFunding int64                  `json:"estimated_funding"` // eg. $25,000,000
	ExpectedAwards   string                 `json:"expected_awards"`   // eg. 10 or N/A
	GrantID          int64                  `json:"grant_id"`          // eg. 347509
	Match            bool                   `json:"match"`
	OppNumber        string                 `json:"opportunity_number"` // eg. USDA-FS-2020-01
	OppTitle         string                 `json:"opportunity_title"`  // eg. "FY 2020 Community Connect Grant Program"
}

// Elegibility for FFIS funding opportunities as presented in FFIS spreadsheets
type FFISFundingEligibility struct {
	HigherEducation bool `json:"higher_education"` // Institutions of Higher Education
	Local           bool `json:"local"`            // Local Governments
	NonProfits      bool `json:"non_profits"`      // Non-profits
	Other           bool `json:"other"`            // Other/see announcement
	State           bool `json:"state"`            // State Governments
	Tribal          bool `json:"tribal"`           // Tribal Governments
}
