package usdr

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwapMap(t *testing.T) {
	t.Run("unique values", func(t *testing.T) {
		src := map[string]int{"A": 1, "B": 2, "C": 3}
		exp := map[int]string{1: "A", 2: "B", 3: "C"}
		assert.Equal(t, exp, swapMap(src))
	})
	t.Run("duplicate values", func(t *testing.T) {
		src := map[string]int{"A": 1, "B": 2, "C": 1}
		assert.PanicsWithError(t,
			"source value creates duplicate key in destination map: 1",
			func() { swapMap(src) })
	})
}

func TestDateType(t *testing.T) {
	t.Run("to json", func(t *testing.T) {
		now := time.Now()
		today := Date(now)
		b, err := json.Marshal(today)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%q", now.Format(DateLayout)), string(b))
	})
	t.Run("from json", func(t *testing.T) {
		now := time.Now()
		today := Date{}
		json.Unmarshal([]byte(fmt.Sprintf("%q", now.Format(DateLayout))), &today)
		assert.Equal(t, now.Year(), time.Time(today).Year())
		assert.Equal(t, now.Month(), time.Time(today).Month())
		assert.Equal(t, now.Day(), time.Time(today).Day())
	})
}

func TestApplicant(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		assert.ErrorIs(t, (&Applicant{}).Validate(), ErrInvalidApplicant)
		for name, code := range applicantCodesByName {
			a := &Applicant{Name: name, Code: code}
			require.NoError(t, a.Validate())
		}
	})
	t.Run("ApplicantFromName", func(t *testing.T) {
		for name, code := range applicantCodesByName {
			a, err := ApplicantFromName(string(name))
			require.NoError(t, err)
			require.Equal(t, name, a.Name)
			require.Equal(t, code, a.Code)
		}
		_, err := ApplicantFromName("invalid")
		require.ErrorIs(t, err, ErrInvalidApplicant)
	})

	t.Run("ApplicantFromCode", func(t *testing.T) {
		for code, name := range applicantNamesByCode {
			a, err := ApplicantFromCode(string(code))
			require.NoError(t, err)
			require.Equal(t, code, a.Code)
			require.Equal(t, name, a.Name)
		}
		_, err := ApplicantFromCode("invalid")
		require.ErrorIs(t, err, ErrInvalidApplicant)
	})
}

func TestFundingActivityCategory(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		for name, code := range fundingActivityCategoryCodesByName {
			f := &FundingActivityCategory{Name: name, Code: code}
			require.NoError(t, f.Validate())
		}
		f := FundingActivityCategory{}
		require.ErrorIs(t, f.Validate(), ErrInvalidFundingActivityCategory)
	})

	t.Run("FundingActivityCategoryFromName", func(t *testing.T) {
		for name, code := range fundingActivityCategoryCodesByName {
			f, err := FundingActivityCategoryFromName(string(name))
			require.NoError(t, err)
			require.Equal(t, name, f.Name)
			require.Equal(t, code, f.Code)
		}
		_, err := FundingActivityCategoryFromName("invalid")
		require.ErrorIs(t, err, ErrInvalidFundingActivityCategory)
	})

	t.Run("FundingActivityCategoryFromCode", func(t *testing.T) {
		for code, name := range fundingActivityCategoryNamesByCode {
			f, err := FundingActivityCategoryFromCode(string(code))
			require.NoError(t, err)
			require.Equal(t, code, f.Code)
			require.Equal(t, name, f.Name)
		}
		_, err := FundingActivityCategoryFromCode("invalid")
		require.ErrorIs(t, err, ErrInvalidFundingActivityCategory)
	})
}

func TestFundingActivity(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		valid := FundingActivity{}
		assert.NoError(t, valid.Validate())
		invalid := FundingActivity{Categories: []FundingActivityCategory{{}}}
		assert.ErrorIs(t, invalid.Validate(), ErrInvalidFundingActivityCategory)
	})
}

func TestFundingInstrument(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		assert.ErrorIs(t, (&FundingInstrument{}).Validate(), ErrInvalidFundingInstrument)
		for name, code := range fundingInstrumentCodesByName {
			fi := &FundingInstrument{Name: name, Code: code}
			require.NoError(t, fi.Validate())
		}
	})
	t.Run("FundingInstrumentFromName", func(t *testing.T) {
		for name, code := range fundingInstrumentCodesByName {
			fi, err := FundingInstrumentFromName(string(name))
			require.NoError(t, err)
			require.Equal(t, name, fi.Name)
			require.Equal(t, code, fi.Code)
		}
		_, err := FundingInstrumentFromName("invalid")
		require.ErrorIs(t, err, ErrInvalidFundingInstrument)
	})

	t.Run("FundingInstrumentFromCode", func(t *testing.T) {
		for code, name := range fundingInstrumentNamesByCode {
			fi, err := FundingInstrumentFromCode(string(code))
			require.NoError(t, err)
			require.Equal(t, name, fi.Name)
			require.Equal(t, code, fi.Code)
		}
		_, err := FundingInstrumentFromCode("invalid")
		require.ErrorIs(t, err, ErrInvalidFundingInstrument)
	})
}

func TestOpportunityCategory(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		assert.ErrorIs(t, (&OpportunityCategory{}).Validate(), ErrInvalidOpportunityCategory)
		for name, code := range opportunityCategoryCodesByName {
			oc := &OpportunityCategory{Name: name, Code: code}
			require.NoError(t, oc.Validate())
		}
	})
	t.Run("OpportunityCategoryFromName", func(t *testing.T) {
		for name, code := range opportunityCategoryCodesByName {
			oc, err := OpportunityCategoryFromName(string(name))
			require.NoError(t, err)
			require.Equal(t, name, oc.Name)
			require.Equal(t, code, oc.Code)
		}
		_, err := OpportunityCategoryFromName("invalid")
		require.ErrorIs(t, err, ErrInvalidOpportunityCategory)
	})

	t.Run("OpportunityCategoryFromCode", func(t *testing.T) {
		for code, name := range opportunityCategoryNamesByCode {
			oc, err := OpportunityCategoryFromCode(string(code))
			require.NoError(t, err)
			require.Equal(t, name, oc.Name)
			require.Equal(t, code, oc.Code)
		}
		_, err := OpportunityCategoryFromCode("invalid")
		require.ErrorIs(t, err, ErrInvalidOpportunityCategory)
	})
}

func TestOpportunityMilestones(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		mil := OpportunityMilestones{}
		assert.EqualError(t, mil.Validate(), "cannot be nil: PostDate")
		today := Date(time.Now())
		mil.PostDate = &today
		assert.NoError(t, mil.Validate())
	})
}

func TestOpportunity(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		o := Opportunity{}
		err := o.Validate()
		require.Error(t, err)
		for _, field := range []string{"Id", "Number", "Title"} {
			assert.ErrorContains(t, err, fmt.Sprintf("cannot be empty: %s", field))
		}
		assert.ErrorContains(t, err, "cannot be nil: LastUpdated")
	})
}

func TestRevision(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		rev := Revision{}
		assert.EqualError(t, rev.Validate(), "cannot be empty: revision id")
		rev.Id = ulid.Make()
		assert.NoError(t, rev.Validate())
	})

	t.Run("json", func(t *testing.T) {
		rev1 := Revision{Id: ulid.Make()}
		b, err := json.Marshal(rev1)
		require.NoError(t, err)
		rev2 := Revision{}
		require.NoError(t, json.Unmarshal(b, &rev2))
		assert.Equal(t, rev1, rev2)
		serial := struct {
			Id        ulid.ULID
			Timestamp time.Time
		}{}
		require.NoError(t, json.Unmarshal(b, &serial))
		assert.Equal(t, rev1.Id, serial.Id)
		assert.EqualValues(t, rev1.Id.Time(), serial.Timestamp.UnixMilli())
	})
}

func assertMultierrorOverlap(t *testing.T, allErrs []error, v interface{ Validate() error }) {
	t.Helper()
	maybeError := v.Validate()
	require.Errorf(t, maybeError, "%T.Validate() did not return an error")
	merr, ok := maybeError.(*multierror.Error)
	require.Truef(t, ok, "error from %T.Validate() is not *multierror.Error", v)
	for _, err := range merr.WrappedErrors() {
		assert.Contains(t, allErrs, err)
	}
}

func TestGrant(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		g := Grant{
			FundingInstrumentTypes: []FundingInstrument{{}},
			FundingActivity:        FundingActivity{Categories: []FundingActivityCategory{{}}},
		}
		topErr, ok := g.Validate().(*multierror.Error)
		require.True(t, ok)
		allErrs := topErr.WrappedErrors()
		assertMultierrorOverlap(t, allErrs, &g.Opportunity)
		assertMultierrorOverlap(t, allErrs, &g.FundingActivity)
		assert.Contains(t, allErrs, g.FundingInstrumentTypes[0].Validate())
		assert.Contains(t, allErrs, g.Revision.Validate())
	})
}

func TestGrantModificationEventVersions(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		assert.NoError(t, (&grantModificationEventVersions{}).Validate())
		ver := grantModificationEventVersions{Previous: &Grant{}, New: &Grant{}}
		topErr, ok := ver.Validate().(*multierror.Error)
		require.True(t, ok)
		assertMultierrorOverlap(t, topErr.WrappedErrors(), ver.Previous)
		assertMultierrorOverlap(t, topErr.WrappedErrors(), ver.New)
	})
}

func TestGrantModificationEvent(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		assert.ErrorIs(t, (&GrantModificationEvent{}).Validate(), ErrUnknonwModificationScenario)
		_, err := NewGrantModificationEvent(nil, nil)
		assert.ErrorIs(t, err, ErrUnknonwModificationScenario)
	})
	t.Run("for create", func(t *testing.T) {
		ev, err := NewGrantModificationEvent(&Grant{}, nil)
		assert.NoError(t, err)
		assert.Equal(t, ev.Type, grantModificationEventTypeCreate)
		assert.Equal(t, ev.Type.String(), EventTypeCreate)
	})
	t.Run("for update", func(t *testing.T) {
		ev, err := NewGrantModificationEvent(&Grant{}, &Grant{})
		assert.NoError(t, err)
		assert.Equal(t, ev.Type, grantModificationEventTypeUpdate)
		assert.Equal(t, ev.Type.String(), EventTypeUpdate)
	})
	t.Run("for delete", func(t *testing.T) {
		ev, err := NewGrantModificationEvent(nil, &Grant{})
		assert.NoError(t, err)
		assert.Equal(t, ev.Type, grantModificationEventTypeDelete)
		assert.Equal(t, ev.Type.String(), EventTypeDelete)
	})
}
