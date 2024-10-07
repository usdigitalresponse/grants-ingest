package main

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrantRecordToDDBConversions(t *testing.T) {
	for _, tt := range []struct {
		record                           grantRecord
		expectedGrantIDItemKeyValue      string
		expectedIsForecastAttributeValue bool
	}{
		{
			opportunity{OpportunityID: "1234", OpportunityNumber: "ABC-1234"},
			"1234",
			false,
		},
		{
			forecast{OpportunityID: "9876", OpportunityNumber: "ZYX-9876"},
			"9876",
			true,
		},
	} {
		t.Run(reflect.TypeOf(tt.record).Name(), func(t *testing.T) {
			t.Run("grant_id item key", func(t *testing.T) {
				itemKeyMap := tt.record.dynamoDBItemKey()
				attr, exists := itemKeyMap["grant_id"]
				require.True(t, exists, "Missing grant_id in DynamoDB item key structure")
				var grantId string
				require.NoError(t, attributevalue.Unmarshal(attr, &grantId),
					"Unexpected error unmarshaling value for grant_id key")
				assert.Equal(t, tt.expectedGrantIDItemKeyValue, grantId,
					"Unexpected value for DynamoDB grant_id item key")
			})

			t.Run("is_forecast attribute", func(t *testing.T) {
				attrMap, err := tt.record.dynamoDBAttributeMap()
				require.NoError(t, err, "Unexpected error getting DDB attribute value map")
				attr, exists := attrMap["is_forecast"]
				require.True(t, exists, "Missing is_forecast attribute in DynamoDB item")
				var isForecast bool
				require.NoError(t, attributevalue.Unmarshal(attr, &isForecast),
					"Unexpected error unmarshaling value for is_forecast attribute")
				assert.Equal(t, tt.expectedIsForecastAttributeValue, isForecast,
					"Unexpected value for DynamoDB is_forecast attribute")
			})
		})
	}
}
