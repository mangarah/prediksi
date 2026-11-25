package pricing

import (
	"testing"
	"time"
)

func params(now time.Time) Params {
	return Params{
		TargetMargin: 0.15,
		MinMargin:    0.05,
		EstCost:      100_000,
		Cost:         5_900_000,
		GradeFactor:  1,
		HalfLifeDays: 30,
		Now:          now,
		Weights:      map[string]float64{"toko": 1, "pesaing": 0.8, "marketplace": 0.6, "penjual": 1},
	}
}

// Contoh PRD: PS5 Slim Disc grade A, P = Rp 7.200.000, target margin 15%,
// estimasi biaya Rp 100.000, dibeli Rp 5.800.000.
func TestPRDExample(t *testing.T) {
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	var obs []Observation
	for i := 0; i < 14; i++ {
		obs = append(obs, Observation{Price: 7_200_000, Source: "toko", Series: SeriesSell, Grade: "A", ObservedAt: now.AddDate(0, 0, -i)})
	}
	obs = append(obs, Observation{Price: 12_900_000, Source: "marketplace", Series: SeriesSell, Grade: "A", ObservedAt: now})
	obs = append(obs, Observation{Price: 6_900_000, Source: "penjual", Series: SeriesBuy, Grade: "A", ObservedAt: now})

	r := Quote(obs, "A", params(now))

	if r.Market != 7_200_000 {
		t.Errorf("market = %d, want 7200000", r.Market)
	}
	if r.BuyLimit != 6_020_000 {
		t.Errorf("buy limit = %d, want 6020000", r.BuyLimit)
	}
	if r.Recommended != 7_200_000 {
		t.Errorf("recommended = %d, want 7200000", r.Recommended)
	}
	if r.MinPrice != 6_200_000 {
		t.Errorf("min price = %d, want 6200000", r.MinPrice)
	}
	if r.Profit != 1_300_000 {
		t.Errorf("profit = %d, want 1300000", r.Profit)
	}
	if r.Count != 14 {
		t.Errorf("count = %d, want 14", r.Count)
	}
	if len(r.Outliers) != 1 || r.Outliers[0].Price != 12_900_000 {
		t.Errorf("outliers = %+v, want one at 12900000", r.Outliers)
	}
	if r.LowData {
		t.Error("low data should be false")
	}
}

func TestGradeFallbackAppliesFactor(t *testing.T) {
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	obs := []Observation{
		{Price: 7_200_000, Source: "toko", Series: SeriesSell, Grade: "A", ObservedAt: now},
		{Price: 7_200_000, Source: "toko", Series: SeriesSell, Grade: "A", ObservedAt: now},
	}
	for i := 0; i < 6; i++ {
		obs = append(obs, Observation{Price: 7_200_000, Source: "toko", Series: SeriesSell, Grade: "B", ObservedAt: now})
	}
	p := params(now)
	p.GradeFactor = 0.9
	p.AccessoryDeduction = 150_000
	p.Cost = 0
	p.EstCost = 0
	r := Quote(obs, "A", p)

	if !r.Adjusted {
		t.Fatal("expected adjusted fallback")
	}
	if r.Market != 6_330_000 {
		t.Errorf("market = %d, want 6330000", r.Market)
	}
	if r.Count != 8 {
		t.Errorf("count = %d, want 8", r.Count)
	}
}

func TestLowDataFlag(t *testing.T) {
	now := time.Now()
	obs := []Observation{
		{Price: 7_000_000, Source: "toko", Series: SeriesSell, Grade: "A", ObservedAt: now},
		{Price: 7_100_000, Source: "toko", Series: SeriesSell, Grade: "A", ObservedAt: now},
	}
	r := Quote(obs, "A", params(now))
	if !r.LowData {
		t.Error("expected low data flag")
	}
	if r.Market != 7_000_000 && r.Market != 7_100_000 {
		t.Errorf("market = %d, want one of the two prices", r.Market)
	}
}
