package main

import (
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrEmailUnrecognizedSender  = errors.New("email has unrecognized sender")
	ErrEmailSpamCheckFailed     = errors.New("email spam check failed")
	ErrEmailVirusCheckFailed    = errors.New("email virus check failed")
	ErrEmailSPFCheckFailed      = errors.New("email SPF check failed")
	ErrEmailFailedToParse       = errors.New("failed to parse email")
	ErrEmailDateFailedToParse   = errors.New("failed to parse email date")
	ErrEmailSenderFailedToParse = errors.New("failed to parse email sender")
)

func parseEmailContents(r io.Reader) (msg *mail.Message, sender *mail.Address, date time.Time, err error) {
	msg, err = mail.ReadMessage(r)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrEmailFailedToParse, err)
		return
	}

	p := mail.AddressParser{}
	sender, err = p.Parse(msg.Header.Get("From"))
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrEmailSenderFailedToParse, err)
		return
	}

	date, err = msg.Header.Date()
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrEmailDateFailedToParse, err)
		return
	}

	return
}

func verifyEmailIsTrusted(msg *mail.Message, sender *mail.Address) error {
	allowedFromDomains := strings.Split(env.AllowedEmailSenders, ",")
	if !emailAddressAllowed(sender.Address, allowedFromDomains...) {
		return ErrEmailUnrecognizedSender
	}
	if err := checkEmailSPF(msg); err != nil {
		return err
	}
	if err := checkEmailSpam(msg); err != nil {
		return err
	}
	if err := checkEmailVirus(msg); err != nil {
		return err
	}
	return nil
}

// checkEmailAddress determines whether a given email address matches one or more items
// in an allow list, which may be populated with a combination of email addresses and domain names.
// Returns true when emailAddress matches an item in allowList, or else returns false
// after match candidates are exhausted.
// Note that this function does NOT determine email address validity or deliverability.
// See normalizeEmailAddress for more information on normalization/comparability behavior.
func emailAddressAllowed(emailAddress string, allowList ...string) bool {
	_, domain, emailAddress := normalizeEmailAddress(emailAddress)
	for _, allowed := range allowList {
		allowed = strings.ToLower(strings.TrimSpace(allowed))

		if strings.Contains(allowed, "@") {
			// Allowed item is an email address – check if normalized values match
			_, _, normalizedAllowed := normalizeEmailAddress(allowed)
			if emailAddress == normalizedAllowed {
				return true
			}
		} else {
			// Check if item matches normalized email address domain
			if domain == allowed {
				return true
			}
		}
	}
	return false
}

// Normalizes an email address for comparability.
// A normalized email address is considered to be the following:
//   - All lowercase
//   - No leading or trailing whitespace
//   - All dot characters (.) removed from the name component
//   - No plus-addressing/mail extension (in the name component)
//
// Returns the normalized name (before @ sign), domain (after @ sign) and fully-normalized values.
func normalizeEmailAddress(addr string) (name, domain, complete string) {
	splitAt := strings.SplitN(strings.ToLower(strings.TrimSpace(addr)), "@", 2)
	name = strings.SplitN(splitAt[0], "+", 2)[0]
	name = strings.ReplaceAll(name, ".", "")
	domain = strings.Join(splitAt[1:], "")
	complete = fmt.Sprintf("%s@%s", name, domain)
	return
}

func checkEmailSPF(msg *mail.Message) error {
	if !strings.HasPrefix(msg.Header.Get("Received-SPF"), "pass") {
		return ErrEmailSPFCheckFailed
	}
	return nil
}

func checkEmailSpam(msg *mail.Message) error {
	if msg.Header.Get("X-SES-Spam-Verdict") != "PASS" {
		return ErrEmailSpamCheckFailed
	}
	return nil
}

func checkEmailVirus(msg *mail.Message) error {
	if msg.Header.Get("X-SES-Virus-Verdict") != "PASS" {
		return ErrEmailVirusCheckFailed
	}
	return nil
}
