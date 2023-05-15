package main

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
	"github.com/xuri/excelize/v2"
)

// Currently, the FFIS spreadsheet uses an "X" to indicate eligibility
func parseEligibility(value string) bool {
	return value == "X"
}

// parseXLSXFile takes a filepath to an xlsx file and parses it into a slice of
// ffis.FFISFundingOpportunity. It will only return opportunities that have a valid
// opportunity number, and will not return an error on individual cell parsing
// issues.
func parseXLSXFile(r io.Reader) ([]ffis.FFISFundingOpportunity, error) {
	xlFile, err := excelize.OpenReader(r)

	if err != nil {
		return nil, err
	}

	defer func() {
		if err := xlFile.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// We assume the excel file only has one sheet
	sheet := "Sheet1"
	rows, err := xlFile.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	var opportunities []ffis.FFISFundingOpportunity

	// Tracks if the iterator has found headers for the sheet
	foundHeaders := false

	// The sheet has rows as categories that we want to assign to
	// each opportunity underneath that category, so we re-assign this
	// as we iterate through the rows
	//
	// TODO: Is this the right name for this metadata?
	opportunityCategory := ""

rowLoop:
	for rowIndex, row := range rows {
		opportunity := ffis.FFISFundingOpportunity{}

		for colIndex, cell := range row {
			// We assume the first column header is "CFDA", if it is,
			// we're in the headers row, so skip it and set the flag that
			// the content follows
			if colIndex == 0 && cell == "CFDA" {
				foundHeaders = true
				continue rowLoop
			}

			// If we have not yet found the headers, skip the row
			if !foundHeaders {
				continue rowLoop
			}

			// Test the first cell, which is likely either a CFDA number,
			// a blank row, or a category (eg Inflation Reduction Act)
			if colIndex == 0 {
				// If the cell is blank, skip the row
				if cell == "" {
					continue rowLoop
				}

				// Test if the cell is a CFDA number, otherwise
				// we can assume if it has content it's a category. Apparently
				// this is a consistent CFDA format based on this page:
				// https://grantsgovprod.wordpress.com/2018/06/04/what-is-a-cfda-number-2/
				r, err := regexp.Compile(`^[0-9]{2}\.[0-9]{3}$`)
				if err != nil {
					return nil, err
				}
				// If we don't match a CFDA number, we assume it's a opportunity category
				// and continue
				if !r.MatchString(cell) {
					opportunityCategory = cell
					continue rowLoop
				}
			}

			// Populate opportunity based on an assumed format
			// where colIndex is a column (zero is A, 1 is B, etc.)
			switch colIndex {
			case 0:
				opportunity.CFDA = cell
			case 1:
				opportunity.OppTitle = cell
			case 2:
				opportunity.Agency = cell
			case 3:
				// If estimated funding is N/A, assume 0
				if cell == "N/A" {
					continue
				}

				num, err := strconv.ParseInt(cell, 10, 64)
				// If we can't parse the funding amount, just skip the column
				if err != nil {
					fmt.Println("Error parsing estimated funding: ", err)
					continue
				}
				opportunity.EstimatedFunding = num
			case 4:
				opportunity.ExpectedAwards = cell
			case 5:
				opportunity.OppNumber = cell

				// cellAxis (eg. A4) is used to get a hyperlink for a cell. We
				// need to increment the index because Excel is not zero-indexed
				cellAxis, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
				if err != nil {
					fmt.Println("Error parsing cell axis for grant ID: ", err)
					continue
				}

				hasLink, target, err := xlFile.GetCellHyperLink(sheet, cellAxis)
				if err != nil {
					// log this, it is not worth aborting the whole extraction for
					fmt.Println("Error getting cell hyperlink for grant ID: ", err)
					continue
				}

				// If we have a link, parse the URL for the opportunity ID which
				// is the only way to get it from the spreadsheet
				if hasLink {
					url, err := url.Parse(target)
					if err != nil {
						fmt.Println("Error parsing link for grant ID: ", err)
						continue
					}

					// The opportunity ID should be a < 20 digit numeric value
					oppID, err := strconv.ParseInt(url.Query().Get("oppId"), 10, 64)
					if err != nil {
						fmt.Println("Error parsing opportunity ID: ", err)
						continue
					}

					opportunity.GrantID = oppID
				}
			case 6:
				opportunity.Eligibility.State = parseEligibility(cell)
			case 7:
				opportunity.Eligibility.Local = parseEligibility(cell)
			case 8:
				opportunity.Eligibility.Tribal = parseEligibility(cell)
			case 9:
				opportunity.Eligibility.HigherEducation = parseEligibility(cell)
			case 10:
				opportunity.Eligibility.NonProfits = parseEligibility(cell)
			case 11:
				opportunity.Eligibility.Other = parseEligibility(cell)
			case 12:
				// If we fail to parse the date, just skip the column
				// and not the whole row
				t, err := time.Parse("1-2-06", cell)
				if err != nil {
					fmt.Println("Error parsing date: ", err)
					continue
				}
				opportunity.DueDate = t
			case 13:
				opportunity.Match = parseEligibility(cell)
			}

			// We will use the most recent category
			opportunity.OppCategory = opportunityCategory
		}

		// Only add valid opportunities
		if opportunity.CFDA != "" {
			opportunities = append(opportunities, opportunity)
		}
	}

	return opportunities, nil
}
