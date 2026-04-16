# cur-cli

A fast, beautiful CLI tool for analysing **AWS Cost and Usage Report (CUR)** Parquet exports directly in your terminal. No spreadsheets, no dashboards — just instant cost breakdowns grouped by service or account, rendered with a styled table powered by [Lip Gloss](https://github.com/charmbracelet/lipgloss).

```
╭────┬──────────────────────────┬────────────┬──────────┬────────────┬────────────╮
│ #  │         Service          │    Cost    │ Currency │ Line Items │ % of Total │
├────┼──────────────────────────┼────────────┼──────────┼────────────┼────────────┤
│ 1  │ Amazon EC2               │ 4321.8700  │ USD      │ 18402      │ 61.2%      │
│ 2  │ Amazon S3                │ 1204.3300  │ USD      │ 5301       │ 17.1%      │
│ 3  │ Amazon RDS               │  987.6600  │ USD      │ 3109       │ 14.0%      │
╰────┴──────────────────────────┴────────────┴──────────┴────────────┴────────────╯
```

---

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Flags](#flags)
- [Implementation Details](#implementation-details)
  - [Project Structure](#project-structure)
  - [parser.go](#parsergo)
  - [aggregator.go](#aggregatorgo)
  - [renderer.go](#renderergo)
  - [main.go](#maingo)
- [Dependencies](#dependencies)

---

## Requirements

- **Go 1.21+**
- An AWS CUR export in **Parquet format** (not CSV). You can configure this in the AWS Billing Console under *Cost & Usage Reports*, selecting Parquet as the export format.

---

## Installation

**Clone and build:**

```bash
git clone https://github.com/LuizCFdosSantos/cur-cli.git
cd cur-cli
go mod tidy
go build -o cur-cli .
```

**Or install directly with Go:**

```bash
go install github.com/LuizCFdosSantos/cur-cli@latest
```

---

## Usage

```bash
# Group costs by service (default), show top 20
./cur-cli --file path/to/cur.parquet

# Show top 10 services
./cur-cli --file path/to/cur.parquet --top 10

# Group by AWS account instead of service
./cur-cli --file path/to/cur.parquet --group account

# Show all entries (no limit)
./cur-cli --file path/to/cur.parquet --top 0

# Short flags work too
./cur-cli -f path/to/cur.parquet -t 5 -g account
```

---

## Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--file` | `-f` | *(required)* | Path to the CUR Parquet file |
| `--top` | `-t` | `20` | Number of top entries to display (`0` = all) |
| `--group` | `-g` | `service` | Grouping dimension: `service` or `account` |

---

## Implementation Details

### Project Structure

```
cur-cli/
├── main.go          # CLI entry point (Cobra)
└── cur/
    ├── parser.go    # Parquet file reading & record extraction
    ├── aggregator.go # Cost aggregation logic
    └── renderer.go  # Terminal table rendering (Lip Gloss)
```

All domain logic lives in the `cur` package, keeping `main.go` thin and focused purely on CLI wiring.

---

### parser.go

**Responsibility:** Opens a CUR Parquet file and returns a flat slice of `Record` structs.

#### Key types

```go
type Record struct {
    ServiceName     string
    Region          string
    AccountId       string
    UsageType       string
    ChargeCategory  string
    Cost            float64
    BillingCurrency string
}
```

`curRow` is an internal struct that mirrors the Parquet schema using struct tags. The tag names must exactly match the column names in the CUR export (e.g. `parquet:"BilledCost"`).

#### How it works

1. Opens the file and calls `parquet.OpenFile` from `github.com/parquet-go/parquet-go`.
2. Validates that the `BilledCost` column exists in the schema before reading — failing fast with a clear error if the file format is unexpected.
3. Uses `parquet.NewGenericReader[curRow]` to read rows in batches of 512, which is efficient for both memory and I/O.
4. Skips any row where `Cost == 0` to filter out credits, refunds, and zero-usage entries.
5. Trims whitespace from all string fields before appending to the result.

A context-aware variant `ParseWithContext` is also provided for use cases where you stream large files from S3 and want to support cancellation mid-read.

---

### aggregator.go

**Responsibility:** Takes the flat `[]Record` slice from the parser and aggregates it into sorted cost summaries.

#### Key type

```go
type ServiceCost struct {
    Service  string
    Cost     float64
    Currency string
    Count    int  // number of line items
}
```

#### Functions

**`ByService(records []Record) []ServiceCost`**

Groups records by a composite key of `(ServiceName, BillingCurrency)`. Using a compound key prevents incorrectly summing costs across different currencies if a multi-currency CUR is ever encountered. Results are sorted by `Cost` descending.

**`ByAccount(records []Record) []ServiceCost`**

Groups records by `AccountId`, reusing the `ServiceCost` type with the account ID in the `Service` field. Useful for understanding cost distribution across linked accounts in an AWS Organization.

**`TotalCost(records []Record) float64`**

A simple fold over all records to produce the grand total, used for computing percentage columns in the renderer.

---

### renderer.go

**Responsibility:** Renders a styled terminal table from a `[]ServiceCost` slice using Lip Gloss.

#### Styles

All styles are defined as package-level variables using `lipgloss.NewStyle()`:

| Variable | Applied to | Description |
|---|---|---|
| `headerStyle` | Column headers | Purple background, bold, centered, light text |
| `rowStyle` | Data rows | Padded, alternating dark background on even rows |
| `footerStyle` | Footer bar | Purple background, bold, right-aligned totals |
| `borderStyle` | Table border | Gray, rounded corners |

#### How the table is built

`lipgloss/table` is used to construct the table declaratively:

```go
t := table.New().
    Border(lipgloss.RoundedBorder()).
    BorderStyle(borderStyle).
    Headers(headers...).
    Rows(rows...).
    StyleFunc(func(row, col int) lipgloss.Style { ... })
```

`StyleFunc` receives a `(row, col int)` and returns the appropriate style, allowing alternating row colours and distinct header/footer treatment in a single pass.

The footer (TOTAL row) is rendered separately as a full-width Lip Gloss block beneath the table, matched to the rendered table width by inspecting the longest line in the output string.

---

### main.go

**Responsibility:** CLI entry point using [Cobra](https://github.com/spf13/cobra).

The root command is defined with `cobra.Command`, binding three flags (`--file`, `--top`, `--group`) to local variables. All logic runs inside `RunE` rather than `Run`, which allows errors to be returned and handled cleanly rather than calling `log.Fatalf` inline.

The execution flow is:

```
cobra parses flags
    → cur.Parse(filePath)         reads & filters Parquet rows
    → cur.TotalCost(records)      computes grand total
    → cur.ByService / ByAccount   aggregates & sorts
    → cur.PrintServiceSummary     renders the table
```

Cobra automatically generates `--help` output from the `Use`, `Short`, and `Long` fields on the command, so no manual usage string is needed.

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework — flags, subcommands, help generation |
| `github.com/parquet-go/parquet-go` | Reading AWS CUR Parquet files |
| `github.com/charmbracelet/lipgloss` | Terminal styling and table rendering |