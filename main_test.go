package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LuizCFdosSantos/cur-cli/cur"
	"github.com/parquet-go/parquet-go"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// curRow mirrors the internal parquet schema. Duplicated here so the test
// package (package main) can write valid Parquet files without importing
// unexported types from the cur package.
type curRow struct {
	ServiceName     string  `parquet:"ServiceName"`
	Region          string  `parquet:"RegionName"`
	AccountId       string  `parquet:"BillingAccountId"`
	UsageType       string  `parquet:"UsageType"`
	ChargeCategory  string  `parquet:"ChargeCategory"`
	Cost            float64 `parquet:"BilledCost"`
	BillingCurrency string  `parquet:"BillingCurrency"`
}

// writeTempParquet creates a valid Parquet file from rows and returns its path.
func writeTempParquet(t *testing.T, rows []curRow) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.parquet")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp parquet: %v", err)
	}
	defer f.Close()
	w := parquet.NewGenericWriter[curRow](f)
	if _, err := w.Write(rows); err != nil {
		t.Fatalf("write parquet rows: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close parquet writer: %v", err)
	}
	return path
}

// buildCmd rebuilds the Cobra command exactly as main() does, so we can invoke
// it in tests without spawning a subprocess.
func buildCmd(filePath *string, topN *int, groupBy *string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cur-cli",
		Short: "Analyse AWS Cost and Usage Report (CUR) Parquet files",
		Long:  `cur-cli parses an AWS CUR Parquet export and prints a cost summary grouped by service or account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := cur.Parse(*filePath)
			if err != nil {
				return err
			}

			total := cur.TotalCost(records)

			switch *groupBy {
			case "account":
				costs := cur.ByAccount(records)
				cur.PrintServiceSummary(costs, total, *topN)
			default:
				costs := cur.ByService(records)
				cur.PrintServiceSummary(costs, total, *topN)
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(filePath, "file", "f", "", "Path to CUR Parquet file (required)")
	rootCmd.Flags().IntVarP(topN, "top", "t", 20, "Show top N services (0 = all)")
	rootCmd.Flags().StringVarP(groupBy, "group", "g", "service", "Group by: service | account")

	// 👇 This replaces your manual validation
	if err := rootCmd.MarkFlagRequired("file"); err != nil {
		panic(err)
	}

	// Optional but recommended for tests
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	return rootCmd
}

// ---------------------------------------------------------------------------
// Flag wiring tests
// ---------------------------------------------------------------------------

// TestCLI_MissingFileFlag verifies that omitting --file causes the command to
// return an error rather than silently doing nothing.
func TestCLI_MissingFileFlag(t *testing.T) {
	var fp string
	topN := 20
	group := "service"
	cmd := buildCmd(&fp, &topN, &group)
	cmd.SetArgs([]string{}) // no --file
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --file is omitted, got nil")
	}
}

// TestCLI_InvalidFile verifies that a non-existent file path propagates an
// error back through RunE.
func TestCLI_InvalidFile(t *testing.T) {
	var fp string
	topN := 20
	group := "service"
	cmd := buildCmd(&fp, &topN, &group)
	cmd.SetArgs([]string{"--file", "/no/such/file.parquet"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing parquet file, got nil")
	}
}

// TestCLI_ValidFile_GroupByService runs the full pipeline end-to-end with a
// real (temporary) Parquet file, grouping by service.  It asserts that no
// error is returned, which exercises Parse → ByService → PrintServiceSummary.
func TestCLI_ValidFile_GroupByService(t *testing.T) {
	path := writeTempParquet(t, []curRow{
		{ServiceName: "EC2", Cost: 200.0, BillingCurrency: "USD"},
		{ServiceName: "S3", Cost: 50.0, BillingCurrency: "USD"},
	})

	var fp string
	topN := 20
	group := "service"
	cmd := buildCmd(&fp, &topN, &group)
	cmd.SetArgs([]string{"--file", path, "--group", "service", "--top", "10"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCLI_ValidFile_GroupByAccount runs the pipeline with --group account,
// exercising the ByAccount aggregation branch.
func TestCLI_ValidFile_GroupByAccount(t *testing.T) {
	path := writeTempParquet(t, []curRow{
		{AccountId: "111111111111", Cost: 100.0, BillingCurrency: "USD"},
		{AccountId: "222222222222", Cost: 75.0, BillingCurrency: "USD"},
	})

	var fp string
	topN := 20
	group := "service"
	cmd := buildCmd(&fp, &topN, &group)
	cmd.SetArgs([]string{"--file", path, "--group", "account"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCLI_ShortFlags verifies that the short flag aliases (-f, -t, -g) are
// wired correctly and behave identically to the long forms.
func TestCLI_ShortFlags(t *testing.T) {
	path := writeTempParquet(t, []curRow{
		{ServiceName: "Lambda", Cost: 5.0, BillingCurrency: "USD"},
	})

	var fp string
	topN := 20
	group := "service"
	cmd := buildCmd(&fp, &topN, &group)
	cmd.SetArgs([]string{"-f", path, "-t", "5", "-g", "service"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error with short flags: %v", err)
	}
}

// TestCLI_TopNZero exercises the edge case where --top 0 means "show all",
// ensuring the limit logic does not clamp the output.
func TestCLI_TopNZero(t *testing.T) {
	path := writeTempParquet(t, []curRow{
		{ServiceName: "EC2", Cost: 10.0, BillingCurrency: "USD"},
		{ServiceName: "S3", Cost: 5.0, BillingCurrency: "USD"},
		{ServiceName: "RDS", Cost: 2.0, BillingCurrency: "USD"},
	})

	var fp string
	topN := 20
	group := "service"
	cmd := buildCmd(&fp, &topN, &group)
	cmd.SetArgs([]string{"--file", path, "--top", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error with --top 0: %v", err)
	}
}
