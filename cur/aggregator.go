package cur

import (
	"sort"
)

// ServiceCost is an aggregated cost entry.
type ServiceCost struct {
	Service  string
	Cost     float64
	Currency string
	Count    int // number of line items
}

// ByService aggregates records and returns totals per service, sorted by cost descending.
func ByService(records []Record) []ServiceCost {
	type key struct{ service, currency string }
	m := make(map[key]*ServiceCost)

	for _, r := range records {
		k := key{r.ServiceName, r.BillingCurrency}
		if v, ok := m[k]; ok {
			v.Cost += r.Cost
			v.Count++
		} else {
			m[k] = &ServiceCost{
				Service:  r.ServiceName,
				Cost:     r.Cost,
				Currency: r.BillingCurrency,
				Count:    1,
			}
		}
	}

	result := make([]ServiceCost, 0, len(m))
	for _, v := range m {
		result = append(result, *v)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Cost > result[j].Cost
	})
	return result
}

// ByAccount aggregates totals per account ID.
func ByAccount(records []Record) []ServiceCost {
	m := make(map[string]*ServiceCost)

	for _, r := range records {
		if v, ok := m[r.AccountId]; ok {
			v.Cost += r.Cost
			v.Count++
		} else {
			m[r.AccountId] = &ServiceCost{
				Service:  r.AccountId, // reuse Service field as label
				Cost:     r.Cost,
				Currency: r.BillingCurrency,
				Count:    1,
			}
		}
	}

	result := make([]ServiceCost, 0, len(m))
	for _, v := range m {
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Cost > result[j].Cost
	})
	return result
}

// TotalCost sums all record costs.
func TotalCost(records []Record) float64 {
	var total float64
	for _, r := range records {
		total += r.Cost
	}
	return total
}
