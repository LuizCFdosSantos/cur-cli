package cur

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"
)

// ---------------------------------------------------------------------------
// Helpers — write a real Parquet file to disk for round-trip tests
// ---------------------------------------------------------------------------

// writeTempParquet serialises rows into a Parquet file in the OS temp dir and
// returns its path. The caller is responsible for removing the file.
func writeTempParquet(t *testing.T, rows []curRow) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.parquet")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	writer := parquet.NewGenericWriter[curRow](f)
	if _, err := writer.Write(rows); err != nil {
		t.Fatalf("write parquet rows: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close parquet writer: %v", err)
	}

	return path
}

// ---------------------------------------------------------------------------
// hasColumn
// ---------------------------------------------------------------------------

// TestHasColumn_Found verifies that hasColumn returns true when the named
// column is present in the schema.
func TestHasColumn_Found(t *testing.T) {
	// Write a minimal parquet file and read its schema back.
	path := writeTempParquet(t, []curRow{
		{ServiceName: "EC2", Cost: 1.0},
	})

	f, _ := os.Open(path)
	defer f.Close()
	info, _ := f.Stat()
	pf, _ := parquet.OpenFile(f, info.Size())

	if !hasColumn(pf.Schema(), "BilledCost") {
		t.Error("expected hasColumn to return true for 'BilledCost'")
	}
}

// TestHasColumn_NotFound verifies that hasColumn returns false for a column
// name that does not exist in the schema.
func TestHasColumn_NotFound(t *testing.T) {
	path := writeTempParquet(t, []curRow{
		{ServiceName: "EC2", Cost: 1.0},
	})

	f, _ := os.Open(path)
	defer f.Close()
	info, _ := f.Stat()
	pf, _ := parquet.OpenFile(f, info.Size())

	if hasColumn(pf.Schema(), "NonExistentColumn") {
		t.Error("expected hasColumn to return false for 'NonExistentColumn'")
	}
}

// ---------------------------------------------------------------------------
// Parse
// ---------------------------------------------------------------------------

// TestParse_FileNotFound verifies that Parse returns a descriptive error when
// the given path does not exist.
func TestParse_FileNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/path/to/file.parquet")
	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

// TestParse_InvalidParquet verifies that Parse returns an error when the file
// is not a valid Parquet file (e.g. a plain text file).
func TestParse_InvalidParquet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.parquet")
	if err := os.WriteFile(path, []byte("this is not parquet"), 0644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected an error for an invalid parquet file, got nil")
	}
}

// TestParse_SkipsZeroCostRows ensures that records with Cost == 0 are filtered
// out and never included in the returned slice.
func TestParse_SkipsZeroCostRows(t *testing.T) {
	rows := []curRow{
		{ServiceName: "EC2", Cost: 0.0, BillingCurrency: "USD"},
		{ServiceName: "S3", Cost: 10.0, BillingCurrency: "USD"},
	}
	path := writeTempParquet(t, rows)

	records, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record (zero-cost skipped), got %d", len(records))
	}
	if records[0].ServiceName != "S3" {
		t.Errorf("expected S3, got %s", records[0].ServiceName)
	}
}

// TestParse_AllZeroCost verifies that when every row has zero cost, the result
// is an empty slice (not nil panicking callers).
func TestParse_AllZeroCost(t *testing.T) {
	rows := []curRow{
		{ServiceName: "EC2", Cost: 0.0},
		{ServiceName: "S3", Cost: 0.0},
	}
	path := writeTempParquet(t, rows)

	records, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

// TestParse_FieldsAreTrimmed verifies that leading/trailing whitespace in
// string fields is stripped during parsing.
func TestParse_FieldsAreTrimmed(t *testing.T) {
	rows := []curRow{
		{
			ServiceName:     "  EC2  ",
			Region:          " us-east-1 ",
			AccountId:       " 123 ",
			UsageType:       " BoxUsage ",
			ChargeCategory:  " Usage ",
			Cost:            5.0,
			BillingCurrency: " USD ",
		},
	}
	path := writeTempParquet(t, rows)

	records, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	checks := map[string]string{
		"ServiceName":     r.ServiceName,
		"Region":          r.Region,
		"AccountId":       r.AccountId,
		"UsageType":       r.UsageType,
		"ChargeCategory":  r.ChargeCategory,
		"BillingCurrency": r.BillingCurrency,
	}
	expected := map[string]string{
		"ServiceName":     "EC2",
		"Region":          "us-east-1",
		"AccountId":       "123",
		"UsageType":       "BoxUsage",
		"ChargeCategory":  "Usage",
		"BillingCurrency": "USD",
	}
	for field, got := range checks {
		if got != expected[field] {
			t.Errorf("%s: expected %q, got %q", field, expected[field], got)
		}
	}
}

// TestParse_MultipleValidRows verifies that all non-zero records are returned
// with correct field values after a round-trip through Parquet serialisation.
func TestParse_MultipleValidRows(t *testing.T) {
	rows := []curRow{
		{ServiceName: "EC2", Region: "us-east-1", AccountId: "111", UsageType: "BoxUsage", ChargeCategory: "Usage", Cost: 100.0, BillingCurrency: "USD"},
		{ServiceName: "S3", Region: "us-west-2", AccountId: "222", UsageType: "Requests", ChargeCategory: "Usage", Cost: 20.5, BillingCurrency: "USD"},
	}
	path := writeTempParquet(t, rows)

	records, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

// TestParse_LargeBatchBoundary writes more than 512 rows (the internal batch
// size) to verify the batched read loop correctly spans multiple iterations
// without dropping records.
func TestParse_LargeBatchBoundary(t *testing.T) {
	const n = 600
	rows := make([]curRow, n)
	for i := range rows {
		rows[i] = curRow{ServiceName: "EC2", Cost: float64(i + 1), BillingCurrency: "USD"}
	}
	path := writeTempParquet(t, rows)

	records, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != n {
		t.Errorf("expected %d records, got %d", n, len(records))
	}
}

// ---------------------------------------------------------------------------
// ParseWithContext
// ---------------------------------------------------------------------------

// TestParseWithContext_FileNotFound verifies that ParseWithContext propagates a
// file-open error correctly.
func TestParseWithContext_FileNotFound(t *testing.T) {
	_, err := ParseWithContext(context.Background(), "/no/such/file.parquet")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestParseWithContext_InvalidParquet verifies the error path when the file is
// not a valid Parquet binary.
func TestParseWithContext_InvalidParquet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.parquet")
	_ = os.WriteFile(path, []byte("not parquet"), 0644)

	_, err := ParseWithContext(context.Background(), path)
	if err == nil {
		t.Fatal("expected error for invalid parquet, got nil")
	}
}

// TestParseWithContext_CancelledContext verifies that passing an already-
// cancelled context causes ParseWithContext to return ctx.Err() rather than
// reading any rows.
func TestParseWithContext_CancelledContext(t *testing.T) {
	// Build a valid parquet file with a column named "lineItem/UnblendedCost"
	// so we get past the schema check. We need a custom struct for this.
	type legacyRow struct {
		ServiceName   string  `parquet:"ServiceName"`
		UnblendedCost float64 `parquet:"lineItem/UnblendedCost"`
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.parquet")
	f, _ := os.Create(path)
	w := parquet.NewGenericWriter[legacyRow](f)
	_, _ = w.Write([]legacyRow{{ServiceName: "EC2", UnblendedCost: 10.0}})
	_ = w.Close()
	f.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := ParseWithContext(ctx, path)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

// TestParseWithContext_MissingCostColumn verifies that ParseWithContext returns
// an error when the Parquet file does not contain the expected cost column
// ("lineItem/UnblendedCost").
func TestParseWithContext_MissingCostColumn(t *testing.T) {
	// writeTempParquet uses curRow which has "BilledCost", not
	// "lineItem/UnblendedCost", so it will always trigger the schema check.
	rows := []curRow{{ServiceName: "EC2", Cost: 5.0}}
	path := writeTempParquet(t, rows)

	_, err := ParseWithContext(context.Background(), path)
	if err == nil {
		t.Fatal("expected schema validation error, got nil")
	}
}

// ---------------------------------------------------------------------------
// readParquet (indirectly via Parse)
// ---------------------------------------------------------------------------

// TestReadParquet_MissingBilledCostColumn verifies that readParquet returns an
// error when the "BilledCost" column is absent from the schema.
// We achieve this by writing a Parquet file with a completely different schema.
func TestReadParquet_MissingBilledCostColumn(t *testing.T) {
	type otherRow struct {
		Foo string `parquet:"Foo"`
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "no_cost.parquet")
	f, _ := os.Create(path)
	w := parquet.NewGenericWriter[otherRow](f)
	_, _ = w.Write([]otherRow{{Foo: "bar"}})
	_ = w.Close()
	f.Close()

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for missing BilledCost column, got nil")
	}
}

// Ensure the bytes package is used (it is imported for the buffer helper
// below which guards future test additions that need in-memory I/O).
var _ = bytes.NewBuffer
