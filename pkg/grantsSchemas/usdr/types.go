package usdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/oklog/ulid/v2"
)

// Utilities

// swapMap creates a new map by swapping the keys and values of a source map.
// Will panic when the values of the source map are not unique.
func swapMap[K, V comparable](source map[K]V) map[V]K {
	res := make(map[V]K)
	for k, v := range source {
		if _, ok := res[v]; ok {
			panic(fmt.Errorf("source value creates duplicate key in destination map: %v", v))
		}
		res[v] = k
	}
	return res
}

// Common types

type Date time.Time

const DateLayout = time.DateOnly

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).Format(DateLayout))
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse(DateLayout, s)
	*d = Date(t)
	return err
}

// AdditionalInformation model

type AdditionalInformation struct {
	Eligibility string `json:"eligibility,omitempty"`
	Text        string `json:"text,omitempty"`
	Url         string `json:"url,omitempty"`
}

// Agency model

type Agency struct {
	Name string `json:"name,omitempty"`
	Code string `json:"code,omitempty"`
}

// Applicant model

type (
	applicantName string
	applicantCode string
)

type Applicant struct {
	Name applicantName `json:"name,omitempty"`
	Code applicantCode `json:"code,omitempty"`
}

func (a *Applicant) Validate() error {
	if code, ok := applicantCodesByName[a.Name]; !ok || a.Code != code {
		return ErrInvalidApplicant
	}
	return nil
}

var (
	ErrInvalidApplicant  = errors.New("invalid applicant")
	applicantNamesByCode = map[applicantCode]applicantName{
		"00": "State governments",
		"01": "County governments",
		"02": "City or township governments",
		"04": "Special district governments",
		"05": "Independent school districts",
		"06": "Public and State controlled institutions of higher education",
		"07": "Native American tribal governments (Federally recognized)",
		"08": "Public housing authorities/Indian housing authorities",
		"11": "Native American tribal organizations (other than Federally recognized tribal governments)",
		"12": "Nonprofits having a 501(c)(3) status with the IRS, other than institutions of higher education",
		"13": "Nonprofits that do not have a 501(c)(3) status with the IRS, other than institutions of higher education",
		"20": "Private institutions of higher education",
		"21": "Individuals",
		"22": "For profit organizations other than small businesses",
		"23": "Small businesses",
		"25": "Others (see text field entitled \"Additional Information on Eligibility\" for clarification)",
		"99": "Unrestricted (i.e., open to any type of entity above), subject to any clarification in text field entitled \"Additional Information on Eligibility\"",
	}
	applicantCodesByName = swapMap(applicantNamesByCode)
)

func ApplicantFromName(name string) (app Applicant, err error) {
	n := applicantName(name)
	c, ok := applicantCodesByName[n]
	app = Applicant{Name: n, Code: c}
	if !ok {
		err = ErrInvalidApplicant
	}
	return
}

func ApplicantFromCode(code string) (app Applicant, err error) {
	c := applicantCode(code)
	n, ok := applicantNamesByCode[c]
	app = Applicant{Name: n, Code: c}
	if !ok {
		err = ErrInvalidApplicant
	}
	return
}

// Award Model

type Award struct {
	Ceiling                      string `json:"ceiling,omitempty"`
	Floor                        string `json:"floor,omitempty"`
	EstimatedTotalProgramFunding string `json:"estimated_total_program_funding,omitempty"`
	ExpectedNumberOfAwards       uint64 `json:"expected_number_of_awards,omitempty"`
}

// CloseDate model

type CloseDate struct {
	Date        *Date  `json:"date,omitempty"`
	Explanation string `json:"explanation,omitempty"`
}

// Email model

type Email struct {
	Address     string `json:"email,omitempty"`
	Description string `json:"description,omitempty"`
}

// FundingActivityCategory model

type (
	fundingActivityCategoryName string
	fundingActivityCategoryCode string
)

type FundingActivityCategory struct {
	Name fundingActivityCategoryName `json:"name,omitempty"`
	Code fundingActivityCategoryCode `json:"code,omitempty"`
}

func (f *FundingActivityCategory) Validate() error {
	if code, ok := fundingActivityCategoryCodesByName[f.Name]; !ok || f.Code != code {
		return ErrInvalidFundingActivityCategory
	}
	return nil
}

var (
	ErrInvalidFundingActivityCategory  = errors.New("invalid funding activity category")
	fundingActivityCategoryNamesByCode = map[fundingActivityCategoryCode]fundingActivityCategoryName{
		"RA":  "Recovery Act",
		"AG":  "Agriculture",
		"AR":  "Arts",
		"BC":  "Business and Commerce",
		"CD":  "Community Development",
		"CP":  "Consumer Protection",
		"DPR": "Disaster Prevention and Relief",
		"ED":  "Education",
		"ELT": "Employment, Labor and Training",
		"EN":  "Energy",
		"ENV": "Environment",
		"FN":  "Food and Nutrition",
		"HL":  "Health",
		"HO":  "Housing",
		"HU":  "Humanities",
		"IIJ": "Infrastructure Investment and Jobs Act",
		"IS":  "Information and Statistics",
		"ISS": "Income Security and Social Services",
		"LJL": "Law, Justice and Legal Services",
		"NR":  "Natural Resources",
		"O":   "Other",
		"OZ":  "Opportunity Zone Benefits",
		"RD":  "Regional Development",
		"ST":  "Science and Technology and Other Research and Development",
		"T":   "Transportation",
		"ACA": "Affordable Care Act",
	}
	fundingActivityCategoryCodesByName = swapMap(fundingActivityCategoryNamesByCode)
)

func FundingActivityCategoryFromName(name string) (cat FundingActivityCategory, err error) {
	n := fundingActivityCategoryName(name)
	c, ok := fundingActivityCategoryCodesByName[n]
	cat = FundingActivityCategory{Name: n, Code: c}
	if !ok {
		err = ErrInvalidFundingActivityCategory
	}
	return
}

func FundingActivityCategoryFromCode(code string) (cat FundingActivityCategory, err error) {
	c := fundingActivityCategoryCode(code)
	n, ok := fundingActivityCategoryNamesByCode[c]
	cat = FundingActivityCategory{Name: n, Code: c}
	if !ok {
		err = ErrInvalidFundingActivityCategory
	}
	return
}

// FundingActivity model

type FundingActivity struct {
	Categories  []FundingActivityCategory `json:"categories,omitempty"`
	Explanation string                    `json:"explanation,omitempty"`
}

func (f *FundingActivity) Validate() error {
	var err *multierror.Error
	for _, c := range f.Categories {
		err = multierror.Append(err, c.Validate())
	}
	return err.ErrorOrNil()
}

// FundingInstrument model

type (
	fundingInstrumentName string
	fundingInstrumentCode string
)

type FundingInstrument struct {
	Name fundingInstrumentName `json:"name,omitempty"`
	Code fundingInstrumentCode `json:"code,omitempty"`
}

func (f *FundingInstrument) Validate() error {
	if code, ok := fundingInstrumentCodesByName[f.Name]; !ok || f.Code != code {
		return ErrInvalidFundingInstrument
	}
	return nil
}

var (
	ErrInvalidFundingInstrument  = errors.New("invalid funding instrument")
	fundingInstrumentNamesByCode = map[fundingInstrumentCode]fundingInstrumentName{
		"CA": "Cooperative Agreement",
		"G":  "Grant",
		"PC": "Procurement Contract",
		"O":  "Other",
	}
	fundingInstrumentCodesByName = swapMap(fundingInstrumentNamesByCode)
)

func FundingInstrumentFromName(name string) (inst FundingInstrument, err error) {
	n := fundingInstrumentName(name)
	c, ok := fundingInstrumentCodesByName[n]
	inst = FundingInstrument{Name: n, Code: c}
	if !ok {
		err = ErrInvalidFundingInstrument
	}
	return
}

func FundingInstrumentFromCode(code string) (inst FundingInstrument, err error) {
	c := fundingInstrumentCode(code)
	n, ok := fundingInstrumentNamesByCode[c]
	inst = FundingInstrument{Name: n, Code: c}
	if !ok {
		err = ErrInvalidFundingInstrument
	}
	return
}

// GrantorContact model

type GrantorContact struct {
	Email Email  `json:"email,omitempty"`
	Text  string `json:"text,omitempty"`
}

// Metadata model

type Metadata struct {
	Version string `json:"version,omitempty"`
}

// OpportunityCategory model

type (
	opportunityCategoryName string
	opportunityCategoryCode string
)

type OpportunityCategory struct {
	Name        opportunityCategoryName `json:"name,omitempty"`
	Code        opportunityCategoryCode `json:"code,omitempty"`
	Explanation string                  `json:"explanation,omitempty"`
}

func (o *OpportunityCategory) Validate() error {
	if code, ok := opportunityCategoryCodesByName[o.Name]; !ok || o.Code != code {
		return ErrInvalidOpportunityCategory
	}
	return nil
}

var (
	ErrInvalidOpportunityCategory  = errors.New("invalid opportunity category")
	opportunityCategoryNamesByCode = map[opportunityCategoryCode]opportunityCategoryName{
		"D": "Discretionary",
		"M": "Mandatory",
		"C": "Continuation",
		"E": "Earmark",
		"O": "Other",
	}
	opportunityCategoryCodesByName = swapMap(opportunityCategoryNamesByCode)
)

func OpportunityCategoryFromName(name string) (cat OpportunityCategory, err error) {
	n := opportunityCategoryName(name)
	c, ok := opportunityCategoryCodesByName[n]
	cat = OpportunityCategory{Name: n, Code: c}
	if !ok {
		err = ErrInvalidOpportunityCategory
	}
	return
}

func OpportunityCategoryFromCode(code string) (cat OpportunityCategory, err error) {
	c := opportunityCategoryCode(code)
	n, ok := opportunityCategoryNamesByCode[c]
	cat = OpportunityCategory{Name: n, Code: c}
	if !ok {
		err = ErrInvalidOpportunityCategory
	}
	return
}

// Opportunity model

type Opportunity struct {
	Id          string                `json:"id,omitempty"`
	Number      string                `json:"number,omitempty"`
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	Category    OpportunityCategory   `json:"category,omitempty"`
	Milestones  OpportunityMilestones `json:"milestones,omitempty"`
	LastUpdated *Date                 `json:"last_updated,omitempty"`
}

func (o *Opportunity) Validate() error {
	err := multierror.Append(o.Category.Validate(), o.Milestones.Validate())
	if o.Id == "" {
		err = multierror.Append(err, fmt.Errorf("cannot be empty: Id"))
	}
	if o.Number == "" {
		err = multierror.Append(err, fmt.Errorf("cannot be empty: Number"))
	}
	if o.Title == "" {
		err = multierror.Append(err, fmt.Errorf("cannot be empty: Title"))
	}
	if o.LastUpdated == nil {
		err = multierror.Append(err, fmt.Errorf("cannot be nil: LastUpdated"))
	}
	return err.ErrorOrNil()
}

// OpportunityMilestones model

type OpportunityMilestones struct {
	PostDate    *Date     `json:"post_date,omitempty"`
	Close       CloseDate `json:"close,omitempty"`
	ArchiveDate *Date     `json:"archive_date,omitempty"`
}

func (o *OpportunityMilestones) Validate() error {
	if o.PostDate == nil {
		return fmt.Errorf("cannot be nil: PostDate")
	}
	return nil
}

// Revision model

type Revision struct {
	Id ulid.ULID `json:"id,omitempty"`
}

func (r *Revision) Validate() error {
	if r.Id.Compare(ulid.ULID{}) <= 0 {
		return fmt.Errorf("cannot be empty: revision id")
	}
	return nil
}

func (r Revision) Time() time.Time {
	u := ulid.ULID(r.Id)
	return ulid.Time(u.Time())
}

func (r Revision) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Id        ulid.ULID `json:"id,omitempty"`
		Timestamp time.Time `json:"timestamp,omitempty"`
	}{
		Id:        r.Id,
		Timestamp: r.Time(),
	})
}

// Grant model

var (
	ValidCFDANumberRegexp = regexp.MustCompile(`^[0-9]{2}[\.][0-9]{3}$`)
	ErrInvalidCFDANumber  = errors.New("invalid CFDA number")
)

type cfdaNumber string

func NewCFDANumber(s string) (v cfdaNumber, err error) {
	v = cfdaNumber(s)
	return v, v.Validate()
}

func (c cfdaNumber) Validate() error {
	if !ValidCFDANumberRegexp.MatchString(string(c)) {
		return ErrInvalidCFDANumber
	}
	return nil
}

type Grant struct {
	FundingInstrumentTypes           []FundingInstrument   `json:"funding_instrument_types,omitempty"`
	CostSharingOrMatchingRequirement *bool                 `json:"cost_sharing_or_matching_requirement,omitempty"`
	CFDANumbers                      []cfdaNumber          `json:"cfda_numbers,omitempty"`
	Bill                             string                `json:"bill,omitempty"`
	EligibleApplicants               []Applicant           `json:"eligible_applicants,omitempty"`
	AdditionalInformation            AdditionalInformation `json:"additional_information,omitempty"`
	Agency                           Agency                `json:"agency,omitempty"`
	Award                            Award                 `json:"award,omitempty"`
	FundingActivity                  FundingActivity       `json:"funding_activity,omitempty"`
	Grantor                          GrantorContact        `json:"grantor,omitempty"`
	Metadata                         Metadata              `json:"metadata,omitempty"`
	Opportunity                      Opportunity           `json:"opportunity,omitempty"`
	Revision                         Revision              `json:"revision,omitempty"`
}

func (g *Grant) Validate() error {
	err := multierror.Append(
		g.Opportunity.Validate(),
		g.Revision.Validate(),
		g.FundingActivity.Validate(),
	)
	for _, cfdaNum := range g.CFDANumbers {
		err = multierror.Append(err, cfdaNum.Validate())
	}
	for _, ea := range g.EligibleApplicants {
		err = multierror.Append(err, ea.Validate())
	}
	for _, fit := range g.FundingInstrumentTypes {
		err = multierror.Append(err, fit.Validate())
	}
	return err.ErrorOrNil()
}

// GrantModificationEvent model

type grantModificationEventType string

func (t grantModificationEventType) String() string {
	return string(t)
}

const (
	EventTypeCreate                  string = "create"
	EventTypeUpdate                  string = "update"
	EventTypeDelete                  string = "delete"
	grantModificationEventTypeCreate        = grantModificationEventType(EventTypeCreate)
	grantModificationEventTypeUpdate        = grantModificationEventType(EventTypeUpdate)
	grantModificationEventTypeDelete        = grantModificationEventType(EventTypeDelete)
)

var ErrUnknonwModificationScenario = errors.New("modification scenario is not one of create, update, delete")

type grantModificationEventVersions struct {
	Previous *Grant `json:"previous"`
	New      *Grant `json:"new"`
}

func (v *grantModificationEventVersions) Validate() error {
	var err *multierror.Error
	if v.Previous != nil {
		err = multierror.Append(err, v.Previous.Validate())
	}
	if v.New != nil {
		err = multierror.Append(err, v.New.Validate())
	}
	return err.ErrorOrNil()
}

type GrantModificationEvent struct {
	Type     grantModificationEventType     `json:"type,omitempty"`
	Versions grantModificationEventVersions `json:"versions,omitempty"`
}

func (e *GrantModificationEvent) Validate() error {
	err := multierror.Append(e.Versions.Validate())
	switch e.Type {
	case grantModificationEventTypeCreate:
	case grantModificationEventTypeUpdate:
	case grantModificationEventTypeDelete:
	default:
		err = multierror.Append(err, ErrUnknonwModificationScenario)
	}
	return err.ErrorOrNil()
}

func NewGrantModificationEvent(newVersion, previousVersion *Grant) (*GrantModificationEvent, error) {
	ev := &GrantModificationEvent{
		Versions: grantModificationEventVersions{
			New:      newVersion,
			Previous: previousVersion,
		},
	}

	if newVersion != nil && previousVersion != nil {
		ev.Type = grantModificationEventTypeUpdate
	} else if newVersion != nil {
		ev.Type = grantModificationEventTypeCreate
	} else if previousVersion != nil {
		ev.Type = grantModificationEventTypeDelete
	} else {
		return nil, ErrUnknonwModificationScenario
	}

	return ev, nil
}
