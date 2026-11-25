package store

import "testing"

func TestMedian(t *testing.T) {
	if got := median([]float64{1, 2, 3}); got != 2 {
		t.Fatalf("median ganjil = %v, want 2", got)
	}
	if got := median([]float64{1, 2, 3, 4}); got != 2.5 {
		t.Fatalf("median genap = %v, want 2.5", got)
	}
	if got := median(nil); got != 0 {
		t.Fatalf("median kosong = %v, want 0", got)
	}
}

func TestPctRank(t *testing.T) {
	vals := []float64{10, 20, 30, 40}
	if got := pctRank(vals, 40, len(vals)); got != 100 {
		t.Fatalf("rank max = %d, want 100", got)
	}
	if got := pctRank(vals, 20, len(vals)); got != 50 {
		t.Fatalf("rank 20 = %d, want 50", got)
	}
	if got := pctRank(nil, 10, 0); got != 0 {
		t.Fatalf("rank kosong = %d, want 0", got)
	}
}
