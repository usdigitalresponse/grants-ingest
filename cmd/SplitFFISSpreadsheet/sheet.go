package main

import (
	"io"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"github.com/usdigitalresponse/grants-ingest/pkg/grantsSchemas/ffis"
	"github.com/xuri/excelize/v2"
)

// Currently, the FFIS spreadsheet uses an "X" to indicate eligibility
func parseEligibility(value string) bool {
	return value == "X"
}

// parseXLSXFile is a function that reads and processes an Excel file stream, converting the data into a slice
// of ffis.FFISFundingOpportunity objects. The file is expected to be provided as an io.Reader.
// The function filters and retains only those funding opportunities that possess a valid grant ID.
//
// Any errors encountered during the parsing of individual cells within the Excel file are not returned as function errors,
// but are instead logged at the WARN level, accompanied by the associated row and column indices for easy identification.
//
// Parameters:
// r: The io.Reader providing the Excel file stream to be parsed.
// logger: The log.Logger used to log any parsing errors at the WARN level.
//
// Returns:
// A slice of ffis.FFISFundingOpportunity objects representing the parsed funding opportunities from the Excel file.
// An error is returned if the parsing process fails at a level beyond individual cell parsing.
func parseXLSXFile(r io.Reader, logger log.Logger) ([]ffis.FFISFundingOpportunity, error) {
	xlFile, err := excelize.OpenReader(r)

	if err != nil {
		return nil, err
	}

	// Used to test if a cell is a CFDA number. Apparently
	// this is a consistent CFDA format based on this page:
	// https://grantsgovprod.wordpress.com/2018/06/04/what-is-a-cfda-number-2/
	cfdaRegex, err := regexp.Compile(`^[0-9]{2}\.[0-9]{3}$`)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := xlFile.Close(); err != nil {
			log.Error(logger, "Error closing excel file", err)
		}
	}()

	// We assume the excel file only has one sheet
	sheet := "Sheet1"

	// Returns all rows in the sheet. Note this assumes the sheet is somewhat limited in
	// size, and will not scale to extremely large worksheets (memory overhead)
	rows, err := xlFile.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	sendMetric("spreadsheet.row_count", float64(len(rows)))
	log.Info(logger, "Parsing spreadsheet", "total_rows", len(rows))

	var opportunities []ffis.FFISFundingOpportunity

	// Tracks if the iterator has found headers for the sheet. A header
	// is a column header, like "CFDA", "Opportunity Title", etc.
	foundHeaders := false

	// The sheet has rows as bills that we want to assign to
	// each opportunity underneath that bill, so we use this
	// as we iterate through the rows. This could be something like
	// "Inflation Reduction Act".
	bill := ""

rowLoop:
	for rowIndex, row := range rows {
		opportunity := ffis.FFISFundingOpportunity{}

		for colIndex, cell := range row {
			logger := log.With(logger, "row_index", row, "column_index", colIndex)

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

				// If we don't match a CFDA number and the row isn't blank, we
				// assume it's a bill and continue
				if !cfdaRegex.MatchString(cell) {
					bill = cell
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
					log.Warn(logger, "Error parsing estimated funding", err)
					sendMetric("spreadsheet.cell_parsing_errors", 1)
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
					log.Warn(logger, "Error parsing cell axis for grant ID", err)
					sendMetric("spreadsheet.cell_parsing_errors", 1)
					continue
				}

				hasLink, target, err := xlFile.GetCellHyperLink(sheet, cellAxis)
				if err != nil {
					// log this, it is not worth aborting the whole extraction for
					log.Warn(logger, "Error getting cell hyperlink for grant ID", err)
					sendMetric("spreadsheet.cell_parsing_errors", 1)
					continue
				}

				// If we have a link, parse the URL for the opportunity ID which
				// is the only way to get it from the spreadsheet
				if hasLink {
					url, err := url.Parse(target)
					if err != nil {
						log.Warn(logger, "Error parsing link for grant ID", err)
						sendMetric("spreadsheet.cell_parsing_errors", 1)
						continue
					}

					// The opportunity ID should be a < 20 digit numeric value
					oppID, err := strconv.ParseInt(url.Query().Get("oppId"), 10, 64)
					if err != nil {
						log.Warn(logger, "Error parsing opportunity ID", err)
						sendMetric("spreadsheet.cell_parsing_errors", 1)
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
					log.Warn(logger, "Error parsing date", err)
					sendMetric("spreadsheet.cell_parsing_errors", 1)
					continue
				}
				opportunity.DueDate = t
			case 13:
				opportunity.Match = parseEligibility(cell)
			}

			// We will use the most recent bill
			opportunity.Bill = bill
		}

		// Only add valid opportunities
		if opportunity.GrantID > 0 {
			opportunities = append(opportunities, opportunity)
		}
	}

	return opportunities, nil
}
