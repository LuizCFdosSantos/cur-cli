package cur

import (
	"strings"

	"github.com/LuizCFdosSantos/goframe/dataframe"
	"github.com/LuizCFdosSantos/goframe/series"
	"github.com/LuizCFdosSantos/goframe/types"
)

// ServiceCost is an aggregated cost entry.
type ServiceCost struct {
	Service  string
	Cost     float64
	Currency string
	Count    int // number of line items
}

// recordsToDF builds a goframe DataFrame from []Record.
// Columns: service_key (ServiceName|BillingCurrency), account, cost, currency, items (1.0 per row for counting).
func recordsToDF(records []Record) (*dataframe.DataFrame, error) {
	n := len(records)
	serviceKeys := make([]string, n)
	accounts := make([]string, n)
	costs := make([]float64, n)
	currencies := make([]string, n)
	ones := make([]float64, n)
	for i, r := range records {
		serviceKeys[i] = r.ServiceName + "|" + r.BillingCurrency
		accounts[i] = r.AccountId
		costs[i] = r.Cost
		currencies[i] = r.BillingCurrency
		ones[i] = 1.0
	}
	return dataframe.New(map[string]*series.Series{
		"service_key": series.FromStrings(serviceKeys, "service_key"),
		"account":     series.FromStrings(accounts, "account"),
		"cost":        series.FromFloats(costs, "cost"),
		"currency":    series.FromStrings(currencies, "currency"),
		"items":       series.FromFloats(ones, "items"),
	}, []string{"service_key", "account", "cost", "currency", "items"})
}

func sumAgg(s *series.Series) types.Value    { return types.Float(s.Sum()) }
func firstStr(s *series.Series) types.Value  {
	if s.Len() > 0 {
		return s.ILoc(0)
	}
	return types.Str("")
}

// ByService aggregates records and returns totals per (service, currency), sorted by cost descending.
func ByService(records []Record) []ServiceCost {
	if len(records) == 0 {
		return nil
	}
	df, err := recordsToDF(records)
	if err != nil {
		panic("goframe: build dataframe: " + err.Error())
	}
	grouped, err := df.GroupBy("service_key", map[string]func(*series.Series) types.Value{
		"cost":  sumAgg,
		"items": sumAgg,
	})
	if err != nil {
		panic("goframe: group by service: " + err.Error())
	}
	sorted, err := grouped.SortBy("cost", false)
	if err != nil {
		panic("goframe: sort by cost: " + err.Error())
	}
	rows, _ := sorted.Shape()
	result := make([]ServiceCost, rows)
	for i := 0; i < rows; i++ {
		row := sorted.ILoc(i)
		key, _ := row["service_key"].AsString()
		parts := strings.SplitN(key, "|", 2)
		service, currency := parts[0], ""
		if len(parts) == 2 {
			currency = parts[1]
		}
		cost, _ := row["cost"].AsFloat()
		count, _ := row["items"].AsFloat()
		result[i] = ServiceCost{
			Service:  service,
			Cost:     cost,
			Currency: currency,
			Count:    int(count),
		}
	}
	return result
}

// ByAccount aggregates totals per account ID, sorted by cost descending.
func ByAccount(records []Record) []ServiceCost {
	if len(records) == 0 {
		return nil
	}
	df, err := recordsToDF(records)
	if err != nil {
		panic("goframe: build dataframe: " + err.Error())
	}
	grouped, err := df.GroupBy("account", map[string]func(*series.Series) types.Value{
		"cost":     sumAgg,
		"items":    sumAgg,
		"currency": firstStr,
	})
	if err != nil {
		panic("goframe: group by account: " + err.Error())
	}
	sorted, err := grouped.SortBy("cost", false)
	if err != nil {
		panic("goframe: sort by cost: " + err.Error())
	}
	rows, _ := sorted.Shape()
	result := make([]ServiceCost, rows)
	for i := 0; i < rows; i++ {
		row := sorted.ILoc(i)
		account, _ := row["account"].AsString()
		cost, _ := row["cost"].AsFloat()
		currency, _ := row["currency"].AsString()
		count, _ := row["items"].AsFloat()
		result[i] = ServiceCost{
			Service:  account,
			Cost:     cost,
			Currency: currency,
			Count:    int(count),
		}
	}
	return result
}

// TotalCost sums all record costs using a goframe Series.
func TotalCost(records []Record) float64 {
	if len(records) == 0 {
		return 0
	}
	costs := make([]float64, len(records))
	for i, r := range records {
		costs[i] = r.Cost
	}
	return series.FromFloats(costs, "cost").Sum()
}
