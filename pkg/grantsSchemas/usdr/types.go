package usdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/oklog/ulid/v2"
)

// Common types

type Date time.Time

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).Format("2006-01-02"))
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

func (a Applicant) Validate() error {
	if code, ok := applicantCodesByName[a.Name]; !ok || a.Code != code {
		return ErrInvalidApplicant
	}
	return nil
}

var (
	ErrInvalidApplicant  = errors.New("invalid applicant")
	applicantCodesByName = map[applicantName]applicantCode{
		"State governments":            "00",
		"County governments":           "01",
		"City or township governments": "02",
		"Special district governments": "04",
		// TODO
	}
	applicantNamesByCode = func() map[applicantCode]applicantName {
		m := map[applicantCode]applicantName{}
		for n, c := range applicantCodesByName {
			m[c] = n
		}
		return m
	}()
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

func (f FundingActivityCategory) Validate() error {
	if code, ok := fundingActivityCategoryCodesByName[f.Name]; !ok || f.Code != code {
		return ErrInvalidFundingActivityCategory
	}
	return nil
}

var (
	ErrInvalidFundingActivityCategory  = errors.New("invalid funding activity category")
	fundingActivityCategoryCodesByName = map[fundingActivityCategoryName]fundingActivityCategoryCode{
		"Recovery Act":          "RA",
		"Agriculture":           "AG",
		"Arts":                  "AR",
		"Business and Commerce": "BC",
		// TODO
	}
	fundingActivityCategoryNamesByCode = func() map[fundingActivityCategoryCode]fundingActivityCategoryName {
		m := map[fundingActivityCategoryCode]fundingActivityCategoryName{}
		for n, c := range fundingActivityCategoryCodesByName {
			m[c] = n
		}
		return m
	}()
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
		err = ErrInvalidApplicant
	}
	return
}

// FundingActivity model

type FundingActivity struct {
	Categories  []FundingActivityCategory `json:"categories,omitempty"`
	Explanation string                    `json:"explanation,omitempty"`
}

func (f FundingActivity) Validate() error {
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

func (f FundingInstrument) Validate() error {
	if code, ok := fundingInstrumentCodesByName[f.Name]; !ok || f.Code != code {
		return ErrInvalidFundingInstrument
	}
	return nil
}

var (
	ErrInvalidFundingInstrument  = errors.New("invalid funding instrument")
	fundingInstrumentCodesByName = map[fundingInstrumentName]fundingInstrumentCode{
		"Cooperative Agreement": "CA",
		"Grant":                 "G",
		"Procurement Contract":  "PC",
		"Other":                 "O",
	}
	fundingInstrumentNamesByCode = func() map[fundingInstrumentCode]fundingInstrumentName {
		m := map[fundingInstrumentCode]fundingInstrumentName{}
		for n, c := range fundingInstrumentCodesByName {
			m[c] = n
		}
		return m
	}()
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

func (o OpportunityCategory) Validate() error {
	if code, ok := opportunityCategoryCodesByName[o.Name]; !ok || o.Code != code {
		return ErrInvalidOpportunityCategory
	}
	return nil
}

var (
	ErrInvalidOpportunityCategory  = errors.New("invalid opportunity category")
	opportunityCategoryCodesByName = map[opportunityCategoryName]opportunityCategoryCode{
		"Discretionary": "D",
		"Mandatory":     "M",
		"Continuation":  "C",
		"Earmark":       "E",
		"Other":         "O",
	}
	opportunityCategoryNamesByCode = func() map[opportunityCategoryCode]opportunityCategoryName {
		m := map[opportunityCategoryCode]opportunityCategoryName{}
		for n, c := range opportunityCategoryCodesByName {
			m[c] = n
		}
		return m
	}()
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
	Category    OpportunityCategory   `json:"category,omitempty"`
	Milestones  OpportunityMilestones `json:"milestones,omitempty"`
	LastUpdated *Date                 `json:"last_updated,omitempty"`
}

func (o Opportunity) Validate() error {
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

func (o OpportunityMilestones) Validate() error {
	if o.PostDate == nil {
		return fmt.Errorf("cannot be nil: PostDate")
	}
	return nil
}

// Revision model
type Revision struct {
	Id ulid.ULID `json:"id,omitempty"`
}

func (r Revision) Validate() error {
	if r.Id.Compare(ulid.ULID{}) > 0 {
		return fmt.Errorf("cannot be empty: revision")
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

type Grant struct {
	Description                      string                `json:"description,omitempty"`
	FundingInstrumentTypes           []FundingInstrument   `json:"funding_instrument_types,omitempty"`
	CostSharingOrMatchingRequirement *bool                 `json:"cost_sharing_or_matching_requirement,omitempty"`
	CFDANumbers                      []string              `json:"cfda_numbers,omitempty"`
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

func (g Grant) Validate() error {
	err := multierror.Append(
		g.Opportunity.Validate(),
		g.Revision.Validate(),
		g.FundingActivity.Validate(),
	)
	for _, fit := range g.FundingInstrumentTypes {
		err = multierror.Append(err, fit.Validate())
	}
	return err.ErrorOrNil()
}

// GrantModificationEvent model

type grantModificationEventType string

const (
	grantModificationEventTypeCreate grantModificationEventType = "create"
	grantModificationEventTypeUpdate grantModificationEventType = "update"
	grantModificationEventTypeDelete grantModificationEventType = "delete"
)

var ErrUnknonwModificationScenario = errors.New("modification scenario is not one of create, update, delete")

type grantModificationEventVersions struct {
	Previous *Grant `json:"previous"`
	New      *Grant `json:"new"`
}

type GrantModificationEvent struct {
	Type     grantModificationEventType     `json:"type,omitempty"`
	Versions grantModificationEventVersions `json:"versions,omitempty"`
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
