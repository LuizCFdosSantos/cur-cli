package cur

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeRecord is a convenience constructor so test cases stay readable.
func makeRecord(service, region, account, usageType, chargeCategory, currency string, cost float64) Record {
	return Record{
		ServiceName:     service,
		Region:          region,
		AccountId:       account,
		UsageType:       usageType,
		ChargeCategory:  chargeCategory,
		Cost:            cost,
		BillingCurrency: currency,
	}
}

// ---------------------------------------------------------------------------
// TotalCost
// ---------------------------------------------------------------------------

// TestTotalCost_Empty ensures that summing an empty slice returns 0 and does
// not panic.
func TestTotalCost_Empty(t *testing.T) {
	got := TotalCost([]Record{})
	if got != 0 {
		t.Errorf("expected 0, got %f", got)
	}
}

// TestTotalCost_SingleRecord verifies that a single record's cost is returned
// unchanged.
func TestTotalCost_SingleRecord(t *testing.T) {
	records := []Record{makeRecord("EC2", "us-east-1", "111", "BoxUsage", "Usage", "USD", 42.5)}
	got := TotalCost(records)
	if got != 42.5 {
		t.Errorf("expected 42.5, got %f", got)
	}
}

// TestTotalCost_MultipleRecords checks that costs from multiple records are
// correctly summed together.
func TestTotalCost_MultipleRecords(t *testing.T) {
	records := []Record{
		makeRecord("EC2", "us-east-1", "111", "BoxUsage", "Usage", "USD", 100.0),
		makeRecord("S3", "us-east-1", "111", "Requests", "Usage", "USD", 50.0),
		makeRecord("RDS", "eu-west-1", "222", "Multi-AZ", "Usage", "USD", 25.5),
	}
	got := TotalCost(records)
	want := 175.5
	if got != want {
		t.Errorf("expected %f, got %f", want, got)
	}
}

// ---------------------------------------------------------------------------
// ByService
// ---------------------------------------------------------------------------

// TestByService_Empty ensures an empty input produces an empty output without
// panicking.
func TestByService_Empty(t *testing.T) {
	result := ByService([]Record{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(result))
	}
}

// TestByService_SingleRecord verifies that a single record is returned as a
// single ServiceCost entry with Count == 1.
func TestByService_SingleRecord(t *testing.T) {
	records := []Record{makeRecord("EC2", "us-east-1", "111", "BoxUsage", "Usage", "USD", 100.0)}
	result := ByService(records)

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Service != "EC2" {
		t.Errorf("expected service EC2, got %s", result[0].Service)
	}
	if result[0].Cost != 100.0 {
		t.Errorf("expected cost 100.0, got %f", result[0].Cost)
	}
	if result[0].Count != 1 {
		t.Errorf("expected count 1, got %d", result[0].Count)
	}
	if result[0].Currency != "USD" {
		t.Errorf("expected currency USD, got %s", result[0].Currency)
	}
}

// TestByService_AggregatesSameService confirms that two records for the same
// service and currency are merged into a single entry with their costs summed
// and Count reflecting both line items.
func TestByService_AggregatesSameService(t *testing.T) {
	records := []Record{
		makeRecord("EC2", "us-east-1", "111", "BoxUsage", "Usage", "USD", 60.0),
		makeRecord("EC2", "eu-west-1", "222", "BoxUsage", "Usage", "USD", 40.0),
	}
	result := ByService(records)

	if len(result) != 1 {
		t.Fatalf("expected 1 merged entry, got %d", len(result))
	}
	if result[0].Cost != 100.0 {
		t.Errorf("expected merged cost 100.0, got %f", result[0].Cost)
	}
	if result[0].Count != 2 {
		t.Errorf("expected count 2, got %d", result[0].Count)
	}
}

// TestByService_SeparatesByCurrency verifies that the same service name billed
// in different currencies is kept as two separate entries, because mixing
// currencies would produce a meaningless total.
func TestByService_SeparatesByCurrency(t *testing.T) {
	records := []Record{
		makeRecord("Support", "us-east-1", "111", "Fee", "Usage", "USD", 10.0),
		makeRecord("Support", "eu-west-1", "222", "Fee", "Usage", "EUR", 9.0),
	}
	result := ByService(records)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries (one per currency), got %d", len(result))
	}
}

// TestByService_SortedDescending checks that results are ordered by cost from
// highest to lowest, regardless of input order.
func TestByService_SortedDescending(t *testing.T) {
	records := []Record{
		makeRecord("S3", "us-east-1", "111", "Requests", "Usage", "USD", 10.0),
		makeRecord("EC2", "us-east-1", "111", "BoxUsage", "Usage", "USD", 500.0),
		makeRecord("RDS", "us-east-1", "111", "Multi-AZ", "Usage", "USD", 200.0),
	}
	result := ByService(records)

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	if result[0].Service != "EC2" {
		t.Errorf("expected EC2 first (highest cost), got %s", result[0].Service)
	}
	if result[1].Service != "RDS" {
		t.Errorf("expected RDS second, got %s", result[1].Service)
	}
	if result[2].Service != "S3" {
		t.Errorf("expected S3 last (lowest cost), got %s", result[2].Service)
	}
}

// TestByService_MultipleServices_MultipleRecords is an integration-style test
// that exercises both aggregation and sorting together across a realistic mix
// of records.
func TestByService_MultipleServices_MultipleRecords(t *testing.T) {
	records := []Record{
		makeRecord("EC2", "us-east-1", "111", "BoxUsage", "Usage", "USD", 100.0),
		makeRecord("S3", "us-east-1", "111", "Requests", "Usage", "USD", 30.0),
		makeRecord("EC2", "us-west-2", "111", "BoxUsage", "Usage", "USD", 150.0),
		makeRecord("Lambda", "us-east-1", "111", "Request", "Usage", "USD", 5.0),
		makeRecord("S3", "eu-west-1", "222", "Storage", "Usage", "USD", 20.0),
	}
	result := ByService(records)

	// EC2: 250, S3: 50, Lambda: 5
	if len(result) != 3 {
		t.Fatalf("expected 3 distinct services, got %d", len(result))
	}

	wantOrder := []struct {
		service string
		cost    float64
		count   int
	}{
		{"EC2", 250.0, 2},
		{"S3", 50.0, 2},
		{"Lambda", 5.0, 1},
	}

	for i, w := range wantOrder {
		if result[i].Service != w.service {
			t.Errorf("[%d] expected service %s, got %s", i, w.service, result[i].Service)
		}
		if result[i].Cost != w.cost {
			t.Errorf("[%d] expected cost %f, got %f", i, w.cost, result[i].Cost)
		}
		if result[i].Count != w.count {
			t.Errorf("[%d] expected count %d, got %d", i, w.count, result[i].Count)
		}
	}
}

// ---------------------------------------------------------------------------
// ByAccount
// ---------------------------------------------------------------------------

// TestByAccount_Empty ensures an empty input produces an empty output without
// panicking.
func TestByAccount_Empty(t *testing.T) {
	result := ByAccount([]Record{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(result))
	}
}

// TestByAccount_SingleRecord verifies a single record maps to one ServiceCost
// entry where the Service field holds the AccountId.
func TestByAccount_SingleRecord(t *testing.T) {
	records := []Record{makeRecord("EC2", "us-east-1", "123456789", "BoxUsage", "Usage", "USD", 99.0)}
	result := ByAccount(records)

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Service != "123456789" {
		t.Errorf("expected Service field to hold account ID '123456789', got %s", result[0].Service)
	}
	if result[0].Cost != 99.0 {
		t.Errorf("expected cost 99.0, got %f", result[0].Cost)
	}
	if result[0].Count != 1 {
		t.Errorf("expected count 1, got %d", result[0].Count)
	}
}

// TestByAccount_AggregatesSameAccount verifies that multiple records belonging
// to the same account are merged into a single entry.
func TestByAccount_AggregatesSameAccount(t *testing.T) {
	records := []Record{
		makeRecord("EC2", "us-east-1", "111111111111", "BoxUsage", "Usage", "USD", 200.0),
		makeRecord("S3", "us-east-1", "111111111111", "Requests", "Usage", "USD", 50.0),
	}
	result := ByAccount(records)

	if len(result) != 1 {
		t.Fatalf("expected 1 merged account entry, got %d", len(result))
	}
	if result[0].Cost != 250.0 {
		t.Errorf("expected merged cost 250.0, got %f", result[0].Cost)
	}
	if result[0].Count != 2 {
		t.Errorf("expected count 2, got %d", result[0].Count)
	}
}

// TestByAccount_SeparatesAccounts confirms that records from different accounts
// remain as separate entries.
func TestByAccount_SeparatesAccounts(t *testing.T) {
	records := []Record{
		makeRecord("EC2", "us-east-1", "111111111111", "BoxUsage", "Usage", "USD", 300.0),
		makeRecord("EC2", "us-east-1", "222222222222", "BoxUsage", "Usage", "USD", 100.0),
	}
	result := ByAccount(records)

	if len(result) != 2 {
		t.Fatalf("expected 2 account entries, got %d", len(result))
	}
}

// TestByAccount_SortedDescending checks that accounts are ordered by total cost
// descending.
func TestByAccount_SortedDescending(t *testing.T) {
	records := []Record{
		makeRecord("EC2", "us-east-1", "account-A", "BoxUsage", "Usage", "USD", 50.0),
		makeRecord("EC2", "us-east-1", "account-B", "BoxUsage", "Usage", "USD", 500.0),
		makeRecord("EC2", "us-east-1", "account-C", "BoxUsage", "Usage", "USD", 150.0),
	}
	result := ByAccount(records)

	if result[0].Service != "account-B" {
		t.Errorf("expected account-B first, got %s", result[0].Service)
	}
	if result[1].Service != "account-C" {
		t.Errorf("expected account-C second, got %s", result[1].Service)
	}
	if result[2].Service != "account-A" {
		t.Errorf("expected account-A last, got %s", result[2].Service)
	}
}
