package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEmailContents(t *testing.T) {
	for _, tt := range []struct {
		name          string
		pathToFixture string
		expError      error
	}{
		{"empty email file", "fixtures/bad_empty.eml", ErrEmailFailedToParse},
		{"invalid email file", "fixtures/bad_data.eml", ErrEmailFailedToParse},
		{"unparseable sender", "fixtures/bad_from.eml", ErrEmailSenderFailedToParse},
		{"missing sender", "fixtures/bad_fromMissing.eml", ErrEmailSenderFailedToParse},
		{"unparseable date", "fixtures/bad_date.eml", ErrEmailDateFailedToParse},
		{"missing date", "fixtures/bad_dateMissing.eml", ErrEmailDateFailedToParse},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := parseEmailContents(getFixture(t, tt.pathToFixture))
			if tt.expError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyEmailIsTrusted(t *testing.T) {
	setupLambdaEnvForTesting(t)

	for _, tt := range []struct {
		name          string
		pathToFixture string
		expError      error
	}{
		{"passes all checks", "fixtures/good.eml", nil},
		{"fail virus check", "fixtures/bad_virus.eml", ErrEmailVirusCheckFailed},
		{"fail spam check", "fixtures/bad_spam.eml", ErrEmailSpamCheckFailed},
		{"fail SPF check", "fixtures/bad_spf.eml", ErrEmailSPFCheckFailed},
		{"fail address check", "fixtures/bad_sender.eml", ErrEmailUnrecognizedSender},
	} {
		t.Run(tt.name, func(t *testing.T) {
			msg, sender, _, err := parseEmailContents(getFixture(t, tt.pathToFixture))
			require.NoError(t, err)

			assert.ErrorIs(t, verifyEmailIsTrusted(msg, sender), tt.expError)
		})
	}
}

func TestCheckEmailAddress(t *testing.T) {
	t.Run("expect allowed", func(t *testing.T) {
		for _, tt := range []struct {
			address   string
			allowList []string
		}{
			{"someone@example.com", []string{"example.com"}},
			{"someone@example.com", []string{"example.com", "example.net"}},
			{"someone@ex.ample.com", []string{"example.com", "example.net", "ex.ample.com"}},
			{"someone@example.com", []string{"person@example.com", "someone@example.com"}},
			{"someone@example.com", []string{"example.net", "example.com"}},
			{"a.b.c@example.xyz", []string{"abc@example.xyz"}},
			{"a.b.c+def@example.xyz", []string{"abc@example.xyz"}},
			{"abc@example.xyz", []string{"abc+q.r.s@example.xyz"}},
			{"a.b.c+def@example.xyz", []string{"abc+q.r.s@example.xyz"}},
			{"some.one+extra@example.com", []string{"example.com"}},
		} {
			assert.True(t, emailAddressAllowed(tt.address, tt.allowList...),
				"Email %q expected to match match allow-list %q", tt.address, tt.allowList)
		}
	})

	t.Run("expect not allowed", func(t *testing.T) {
		for _, tt := range []struct {
			address   string
			allowList []string
		}{
			{"someone@example.com", []string{"example.net"}},
			{"someone@example.com", []string{"another@example.com"}},
			{"someone@example.com", []string{"example.net", "example.org"}},
			{"some.one@example.com", []string{"example.net", "example.org"}},
		} {
			assert.False(t, emailAddressAllowed(tt.address, tt.allowList...),
				"Email %q unexpectedly matched by allow-list %q", tt.address, tt.allowList)
		}
	})
}
