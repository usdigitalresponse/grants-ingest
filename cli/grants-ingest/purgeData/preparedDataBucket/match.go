package preparedDataBucket

import (
	"path/filepath"
	"regexp"
)

var (
	FFISMatcher      = &RegexMatcher{regexp.MustCompile(`\d{3}/\d{3,}/ffis.org/v1.json`)}
	GrantsGovMatcher = &RegexMatcher{regexp.MustCompile(`\d{3}/\d{3,}/grants.gov/v2.xml`)}
	AllMatcher       = &RegexMatcher{regexp.MustCompile(`.*`)}
)

type Matcher interface {
	SetPattern(string) error
	Pattern() string
	Match(string) bool
}

type RegexMatcher struct {
	expr *regexp.Regexp
}

func (m *RegexMatcher) IsSet() bool {
	return m != nil && m.expr != nil
}

func (m *RegexMatcher) Match(key string) bool {
	return m.expr.FindString(key) == key
}

func (m *RegexMatcher) SetPattern(pattern string) error {
	expr, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	m.expr = expr
	return nil
}

func (m *RegexMatcher) Pattern() string {
	return m.expr.String()
}

func (m *RegexMatcher) UnmarshalText(text []byte) error {
	if len(text) > 0 {
		return m.SetPattern(string(text))
	}
	return nil
}

type FilePathMatcher struct {
	pattern string
}

func (m *FilePathMatcher) IsSet() bool {
	return m != nil && m.pattern != ""
}

func (m *FilePathMatcher) Match(key string) bool {
	isMatch, _ := filepath.Match(m.pattern, key)
	return isMatch
}

func (m *FilePathMatcher) SetPattern(pattern string) error {
	_, err := filepath.Match(pattern, "")
	if err != nil {
		return err
	}
	m.pattern = pattern
	return nil
}

func (m *FilePathMatcher) Pattern() string {
	return m.pattern
}

func (m *FilePathMatcher) UnmarshalText(text []byte) (err error) {
	if len(text) > 0 {
		return m.SetPattern(string(text))
	}
	return nil
}
