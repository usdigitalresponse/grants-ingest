package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/oklog/ulid/v2"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	grantsgov "github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/grants.gov"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/usdr"
)

func malformattedField(name string, err error) {
	logger := log.With(logger, "field", name)
	if err != nil {
		logger = log.With(logger, "error", err)
	}
	log.Warn(logger, "Could not parse field")
	sendMetric("item_image.malformatted_field", 1, fmt.Sprintf("field:%s", name))
}

const GrantsGovDateLayout = grantsgov.TimeLayoutMMDDYYYYType

func toPointer[T any](v T) *T {
	return &v
}

type ItemMapper struct {
	attrs map[string]events.DynamoDBAttributeValue
}

func NewItemMapper(m map[string]events.DynamoDBAttributeValue) *ItemMapper {
	return &ItemMapper{m}
}

func (im *ItemMapper) stringFor(k string) (s string) {
	if !im.attrs[k].IsNull() {
		s = im.attrs[k].String()
	}
	return
}

func (im *ItemMapper) timeFor(k string, layout string) (*time.Time, error) {
	if attr := im.attrs[k]; !attr.IsNull() {
		dateString := attr.String()
		if dateString != "" {
			t, err := time.Parse(layout, dateString)
			return &t, err
		}
	}
	return nil, nil
}

func (im *ItemMapper) Grant() usdr.Grant {
	grant := usdr.Grant{
		Bill:                   im.stringFor("Bill"),
		Revision:               im.Revision(),
		Opportunity:            im.Opportunity(),
		FundingActivity:        im.FundingActivity(),
		FundingInstrumentTypes: im.FundingInstruments(),
		Award:                  im.Award(),
		Metadata: usdr.Metadata{
			Version: im.stringFor("Version"),
		},
		Agency: usdr.Agency{
			Name: im.stringFor("AgencyName"),
			Code: im.stringFor("AgencyCode"),
		},
		AdditionalInformation: usdr.AdditionalInformation{
			Eligibility: im.stringFor("AdditionalInformationOnEligibility"),
			Text:        im.stringFor("AdditionalInformationText"),
			Url:         im.stringFor("AdditionalInformationURL"),
		},
		Grantor: usdr.GrantorContact{
			Email: usdr.Email{
				Address:     im.stringFor("GrantorContactEmail"),
				Description: im.stringFor("GrantorContactEmailDescription"),
			},
			Text: im.stringFor("GrantorContactText"),
		},
	}

	if req := im.stringFor("CostSharingOrMatchingRequirement"); req != "" {
		yesNo := strings.ToLower(req)
		if yesNo == "yes" {
			grant.CostSharingOrMatchingRequirement = toPointer(true)
		} else if yesNo == "no" {
			grant.CostSharingOrMatchingRequirement = toPointer(false)
		} else {
			malformattedField(
				"CostSharingOrMatchingRequirement",
				fmt.Errorf("value not one of yes or no: %s", yesNo),
			)
		}
	}

	return grant
}

func (im *ItemMapper) Revision() usdr.Revision {
	id, err := ulid.ParseStrict(im.stringFor("revision"))
	if err != nil {
		malformattedField("revision", err)
	}
	return usdr.Revision{Id: id}
}

func (im *ItemMapper) Award() usdr.Award {
	award := usdr.Award{
		Ceiling:                      im.stringFor("AwardCeiling"),
		Floor:                        im.stringFor("AwardFloor"),
		EstimatedTotalProgramFunding: im.stringFor("EstimatedTotalProgramFunding"),
	}
	if exp := im.stringFor("ExpectedNumberOfAwards"); exp != "" {
		val, err := strconv.Atoi(exp)
		if err != nil {
			malformattedField("ExpectedNumberOfAwards", err)
		} else {
			award.ExpectedNumberOfAwards = uint64(val)
		}
	}
	return award
}

func (im *ItemMapper) FundingActivity() usdr.FundingActivity {
	fundingActivity := usdr.FundingActivity{
		Explanation: im.stringFor("CategoryExplanation"),
	}
	if attr := im.attrs["CategoryOfFundingActivity"]; !attr.IsNull() {
		fundingActivity.Categories = make([]usdr.FundingActivityCategory, 0)
		for _, av := range attr.List() {
			category, err := usdr.FundingActivityCategoryFromCode(av.String())
			fundingActivity.Categories = append(fundingActivity.Categories, category)
			if err != nil {
				malformattedField("CategoryOfFundingActivity", err)
			}
		}
	}
	return fundingActivity
}

func (im *ItemMapper) Opportunity() usdr.Opportunity {
	opportunity := usdr.Opportunity{
		Id:          im.stringFor("OpportunityID"),
		Number:      im.stringFor("OpportunityNumber"),
		Title:       im.stringFor("OpportunityTitle"),
		Description: im.stringFor("Description"),
		Milestones:  im.OpportunityMilestones(),
	}

	if attr := im.stringFor("OpportunityCategory"); attr != "" {
		var err error
		opportunity.Category, err = usdr.OpportunityCategoryFromCode(attr)
		if err != nil {
			malformattedField("OpportunityCategory", err)
		}
	}
	opportunity.Category.Explanation = im.stringFor("OpportunityCategoryExplanation")

	if parsed, err := im.timeFor("LastUpdatedDate", GrantsGovDateLayout); err != nil {
		malformattedField("LastUpdatedDate", err)
	} else {
		opportunity.LastUpdated = (*usdr.Date)(parsed)
	}

	return opportunity
}

func (im *ItemMapper) OpportunityMilestones() usdr.OpportunityMilestones {
	lifecycle := usdr.OpportunityMilestones{}
	if parsed, err := im.timeFor("PostDate", GrantsGovDateLayout); err != nil {
		malformattedField("PostDate", err)
	} else {
		lifecycle.PostDate = (*usdr.Date)(parsed)
	}

	if parsed, err := im.timeFor("ArchiveDate", GrantsGovDateLayout); err != nil {
		malformattedField("ArchiveDate", err)
	} else {
		lifecycle.ArchiveDate = (*usdr.Date)(parsed)
	}

	lifecycle.Close.Explanation = im.stringFor("CloseDateExplanation")
	if parsed, err := im.timeFor("CloseDate", GrantsGovDateLayout); err != nil {
		malformattedField("CloseDate", err)
	} else {
		lifecycle.Close.Date = (*usdr.Date)(parsed)
	}

	return lifecycle
}

func (im *ItemMapper) FundingInstruments() []usdr.FundingInstrument {
	fundingInstruments := make([]usdr.FundingInstrument, 0)
	if attr := im.attrs["FundingInstrumentType"]; !attr.IsNull() {
		for _, val := range attr.List() {
			fundingInstrument, err := usdr.FundingInstrumentFromCode(val.String())
			fundingInstruments = append(fundingInstruments, fundingInstrument)
			if err != nil {
				malformattedField("FundingInstrumentType", err)
			}
		}
	}
	return fundingInstruments
}

// GuardPanic wraps any zero-argument function that returns a single value,
// which may panic when called as part of its normal behavior.
//
// This function is provided because the documented behavior of many accessor methods for the
// events.DynamoDBAttributeValue type is to panic when the stored value of the attribute is
// of a different type than what is expected by the accessor method (for example, caling .Boolean()
// on an attribute with a stored type of StringSet ("SS")). Therefore, this function may be used
// to wrap calls to various functions/methods which make use of DynamoDBAttribute value accessors,
// such as those provided by ItemMapper.
//
// If the wrapped function panics, GuardPanic recovers and returns an error
// representing the panic value according to the following behavior:
//   - If the recovered panic value is an error, returns the error as is.
//   - If the recovered panic value is a string, returns an error created from that string.
//   - If the recovered panic value is any other type, returns an error prefixed with
//     "unknown panic:", followed by the verbose string representation of the value and its type.
func GuardPanic[T any](wrappedFunc func() T) (t T, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("unknown panic: %+v of type %T", r, r)
			}
		}
	}()
	res := wrappedFunc()
	return res, err
}
