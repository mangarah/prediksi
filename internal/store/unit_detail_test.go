package store

import "testing"

func TestSuggestPrice(t *testing.T) {
	cases := []struct{ listed, min, want int64 }{
		{7_200_000, 6_700_000, 6_840_000},
		{7_200_000, 6_900_000, 6_900_000},
		{1_000_000, 0, 950_000},
		{0, 500_000, 500_000},
	}
	for _, c := range cases {
		if got := SuggestPrice(c.listed, c.min); got != c.want {
			t.Errorf("SuggestPrice(%d, %d) = %d, want %d", c.listed, c.min, got, c.want)
		}
	}
}
