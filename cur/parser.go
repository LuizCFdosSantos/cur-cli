package cur

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/parquet-go/parquet-go"
)

// Record holds the fields we care about from each CUR line.
type Record struct {
	ServiceName     string
	Region          string
	AccountId       string
	UsageType       string
	ChargeCategory  string
	Cost            float64
	BillingCurrency string
}

// curRow mirrors the Parquet schema for the CUR columns we care about.
// Tags must match the exact Parquet column names in the CUR export.
type curRow struct {
	ServiceName     string  `parquet:"ServiceName"`
	Region          string  `parquet:"RegionName"`
	AccountId       string  `parquet:"BillingAccountId"`
	UsageType       string  `parquet:"UsageType"`
	ChargeCategory  string  `parquet:"ChargeCategory"`
	Cost            float64 `parquet:"BilledCost"`
	BillingCurrency string  `parquet:"BillingCurrency"`
}

// Parse reads a CUR Parquet file and returns all non-zero-cost records.
func Parse(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("open parquet: %w", err)
	}

	return readParquet(pf)
}

func readParquet(pf *parquet.File) ([]Record, error) {
	// Validate that the cost column exists in the schema.
	schema := pf.Schema()
	if !hasColumn(schema, "BilledCost") {
		return nil, fmt.Errorf("cost column (BilledCost) not found in parquet schema")
	}

	reader := parquet.NewGenericReader[curRow](pf)
	defer reader.Close()

	var records []Record
	batch := make([]curRow, 512) // read in batches for efficiency

	for {
		n, err := reader.Read(batch)
		for _, row := range batch[:n] {
			if row.Cost == 0 {
				continue // skip zero-cost rows and credits
			}
			records = append(records, Record{
				ServiceName:     strings.TrimSpace(row.ServiceName),
				Region:          strings.TrimSpace(row.Region),
				AccountId:       strings.TrimSpace(row.AccountId),
				UsageType:       strings.TrimSpace(row.UsageType),
				ChargeCategory:  strings.TrimSpace(row.ChargeCategory),
				Cost:            row.Cost,
				BillingCurrency: strings.TrimSpace(row.BillingCurrency),
			})
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read parquet rows: %w", err)
		}
	}

	return records, nil
}

// hasColumn checks if a column name exists anywhere in the Parquet schema.
func hasColumn(schema *parquet.Schema, name string) bool {
	for _, field := range schema.Fields() {
		if field.Name() == name {
			return true
		}
	}
	return false
}

// ParseWithContext is a context-aware variant — useful for large files
// where you may want to cancel mid-read (e.g. from an S3 stream).
func ParseWithContext(ctx context.Context, path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("open parquet: %w", err)
	}

	schema := pf.Schema()
	if !hasColumn(schema, "lineItem/UnblendedCost") {
		return nil, fmt.Errorf("cost column (lineItem/UnblendedCost) not found in parquet schema")
	}

	reader := parquet.NewGenericReader[curRow](pf)
	defer reader.Close()

	var records []Record
	batch := make([]curRow, 512)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		n, err := reader.Read(batch)
		for _, row := range batch[:n] {
			if row.Cost == 0 {
				continue
			}
			records = append(records, Record{
				ServiceName:     strings.TrimSpace(row.ServiceName),
				Region:          strings.TrimSpace(row.Region),
				AccountId:       strings.TrimSpace(row.AccountId),
				UsageType:       strings.TrimSpace(row.UsageType),
				ChargeCategory:  strings.TrimSpace(row.ChargeCategory),
				Cost:            row.Cost,
				BillingCurrency: strings.TrimSpace(row.BillingCurrency),
			})
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read parquet rows: %w", err)
		}
	}

	return records, nil
}
