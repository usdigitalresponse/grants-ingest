package awsHelpers

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/oklog/ulid/v2"
)

var ErrEmptyFields = errors.New("cannot generate a conditional expression for empty fields")

// DDBSetRevisionForUpdate adds a DynamoDB SET operation to an UpdateBuilder, which
// sets the value of an item's "revision" attribute to a freshly-generated ULID string.
func DDBSetRevisionForUpdate(builder expression.UpdateBuilder) expression.UpdateBuilder {
	return builder.Set(expression.Name("revision"), expression.Value(ulid.Make().String()))
}

// DDBIfAnyValueChangedCondition creates a conditional update expression that will only allow
// a table item to update if one of the field values provided in ifAttributeValuesChanged is different
// than the currently-stored values. This facilitates updating certain attributes (not included
// in ifAttributeValuesChanged) only when when a subset of attributes (which are included in
// ifAttributeValuesChanged) actually have updates.
// The primary use-case for this functionality is managing a revision identifier attribute,
// which must be updated only when at least one other item attribute is modified, but should
// never be the sole update to an existing item.
//
// Returns ErrEmptyFields when ifAttributeValuesChanged is an empty map.
func DDBIfAnyValueChangedCondition(ifAttributeValuesChanged map[string]types.AttributeValue) (expression.ConditionBuilder, error) {
	if len(ifAttributeValuesChanged) == 0 {
		return expression.ConditionBuilder{}, ErrEmptyFields
	}

	builders := []expression.ConditionBuilder{}
	for k, v := range ifAttributeValuesChanged {
		builders = append(builders, expression.Name(k).NotEqual(expression.Value(v)))
	}

	condition := builders[0]
	if len(builders) > 1 {
		condition = condition.Or(builders[1], builders[2:]...)
	}
	return condition, nil
}
