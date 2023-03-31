wget -q -O schema.xsd https://apply07.grants.gov/apply/system/schemas/OpportunityDetail-V1.0.xsd &&\
xsdgen -o types.go -pkg grantsgov schema.xsd &&\
sed -i '' -e 's?http://apply.grants.gov/system/OpportunityDetail-V1.0\ ??g' types.go
