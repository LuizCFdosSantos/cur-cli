package main

import (
	"fmt"
	"log"
	"os"

	"github.com/LuizCFdosSantos/cur-cli/cur"
	"github.com/spf13/cobra"
)

func main() {
	var (
		filePath string
		topN     int
		groupBy  string
	)

	rootCmd := &cobra.Command{
		Use:   "cur-cli",
		Short: "Analyse AWS Cost and Usage Report (CUR) Parquet files",
		Long: `cur-cli parses an AWS CUR Parquet export and prints a cost
summary grouped by service or account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}

			fmt.Printf("Parsing %s ...\n", filePath)

			records, err := cur.Parse(filePath)
			if err != nil {
				return fmt.Errorf("error parsing CUR: %w", err)
			}

			fmt.Printf("Loaded %d billing line items.\n", len(records))

			total := cur.TotalCost(records)

			switch groupBy {
			case "account":
				costs := cur.ByAccount(records)
				fmt.Printf("\n=== Cost by Account (top %d) ===\n", topN)
				cur.PrintServiceSummary(costs, total, topN)
			default:
				costs := cur.ByService(records)
				fmt.Printf("\n=== Cost by Service (top %d) ===\n", topN)
				cur.PrintServiceSummary(costs, total, topN)
			}

			return nil
		},
	}

	rootCmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to CUR Parquet file (required)")
	rootCmd.Flags().IntVarP(&topN, "top", "t", 20, "Show top N services (0 = all)")
	rootCmd.Flags().StringVarP(&groupBy, "group", "g", "service", "Group by: service | account")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
