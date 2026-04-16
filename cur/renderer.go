package cur

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	purple    = lipgloss.Color("99")
	gray      = lipgloss.Color("245")
	lightGray = lipgloss.Color("236")

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(purple).
			Bold(true).
			Align(lipgloss.Center).
			Padding(0, 1)

	rowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true).
			Padding(0, 1).
			Align(lipgloss.Right)

	borderStyle = lipgloss.NewStyle().
			Foreground(gray)
)

// PrintServiceSummary renders a styled cost table to stdout using Lip Gloss.
func PrintServiceSummary(costs []ServiceCost, total float64, topN int) {
	limit := len(costs)
	if topN > 0 && topN < limit {
		limit = topN
	}

	headers := []string{"#", "Service", "Cost", "Currency", "Line Items", "% of Total"}

	rows := make([][]string, 0, limit)
	for i, sc := range costs[:limit] {
		pct := 0.0
		if total > 0 {
			pct = (sc.Cost / total) * 100
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			sc.Service,
			fmt.Sprintf("%.4f", sc.Cost),
			sc.Currency,
			fmt.Sprintf("%d", sc.Count),
			fmt.Sprintf("%.1f%%", pct),
		})
	}

	footerValues := []string{
		"",
		"TOTAL",
		fmt.Sprintf("%.4f", total),
		"",
		"",
		"100.0%",
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row == len(rows): // footer row
				return footerStyle
			case row%2 == 0:
				return rowStyle.Background(lightGray)
			default:
				return rowStyle
			}
		})

	fmt.Println()
	fmt.Println(t.Render())

	// Render footer as a separate styled row beneath the table
	fmt.Println(renderFooter(footerValues, t.Render()))
	fmt.Println()
}

// renderFooter builds a simple styled footer line matching the table width.
func renderFooter(values []string, renderedTable string) string {
	// Find the width of the rendered table to match it
	lines := strings.Split(renderedTable, "\n")
	width := 0
	for _, l := range lines {
		if len(l) > width {
			width = len(l)
		}
	}

	footer := footerStyle.
		Width(width).
		Background(purple).
		Foreground(lipgloss.Color("229")).
		Render(strings.Join(values, "  "))

	return footer
}
