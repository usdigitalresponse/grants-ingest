package awsHelpers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDDBSetRevisionForUpdate(t *testing.T) {
	expr, err := expression.NewBuilder().WithUpdate(
		DDBSetRevisionForUpdate(expression.UpdateBuilder{})).Build()
	require.NoError(t, err)

	rawUpdate := strings.TrimSpace(*expr.Update())
	assert.Equal(t, "SET #0 = :0", rawUpdate)
	assert.Regexp(t,
		"^SET revision = [0-37][0-9A-HJKMNP-TV-Z]{25}$",
		renderDDBExpression(t, rawUpdate, expr),
		"Expression does not seem to set 'revision' field to ULID string")
}

func TestDDBIfAnyValueChangedCondition(t *testing.T) {
	t.Run("empty map returns error", func(t *testing.T) {
		_, err := DDBIfAnyValueChangedCondition(map[string]types.AttributeValue{})
		require.ErrorIs(t, err, ErrEmptyFields)
	})
	for _, tt := range []struct {
		name                   string
		attrs                  map[string]string
		expConditionExpression string
	}{
		{
			name:                   "single conditional expression",
			attrs:                  map[string]string{"foo": "bar"},
			expConditionExpression: "#0 <> :0",
		},
		{
			name:                   "multiple conditional expressions",
			attrs:                  map[string]string{"abc": "xyz", "def": "uvw", "ghi": "rst"},
			expConditionExpression: "(#0 <> :0) OR (#1 <> :1) OR (#2 <> :2)",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attrs, err := attributevalue.MarshalMap(tt.attrs)
			require.NoError(t, err)

			cb, err := DDBIfAnyValueChangedCondition(attrs)
			require.NoError(t, err)
			expr, err := expression.NewBuilder().WithCondition(cb).Build()
			assert.NoError(t, err)

			rawCondition := strings.TrimSpace(*expr.Condition())
			assert.Equal(t, tt.expConditionExpression, rawCondition)
			assert.Equal(t, len(attrs)-1, strings.Count(rawCondition, " OR "),
				"Unexpected count of OR operators in expression %q", *expr.Condition())

			condS := renderDDBExpression(t, rawCondition, expr)
			for k, v := range tt.attrs {
				expectedSub := fmt.Sprintf("%s <> %s", k, v)
				if len(attrs) > 1 {
					expectedSub = fmt.Sprintf("(%s)", expectedSub)
				}
				assert.Contains(t, condS, expectedSub,
					"Expected sub-expression not found in condition string")
			}
		})
	}
}

// Test helper that renders a DynamoDB expression string, replacing expression attribute
// name/value placeholders with their literal forms.
//
// NOTE: This currently only works with string-type attribute values, although support for
// additional types is possible.
//
// Example:
//
//	// Let t be a test and expr be a built expression.Expression representing "SET #1 = :1".
//	fmt.Println(renderDDBExpression(t, *expr.Update(), expr))
//	"SET myKey = something"
func renderDDBExpression(t *testing.T, s string, e expression.Expression) string {
	t.Helper()
	for placeholder, attributeValue := range e.Values() {
		vs, ok := attributeValue.(*types.AttributeValueMemberS)
		require.True(t, ok)
		s = strings.ReplaceAll(s, placeholder, vs.Value)
	}
	for placeholder, name := range e.Names() {
		s = strings.ReplaceAll(s, placeholder, name)
	}
	return s
}
