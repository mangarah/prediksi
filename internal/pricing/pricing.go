// Package pricing menghitung harga pasar, batas beli, dan harga jual dari data pembanding.
package pricing

import (
	"math"
	"sort"
	"time"
)

const (
	SeriesSell = "sell"
	SeriesBuy  = "buy"
)

type Observation struct {
	Price       int64
	Source      string
	Series      string
	Grade       string
	ObservedAt  time.Time
	Accessories []string
	Weight      float64
}

type Params struct {
	TargetMargin       float64 // mis. 0.12 untuk konsol
	MinMargin          float64 // batas kasir, default 0.05
	EstCost            int64   // estimasi biaya tambahan (cleaning/servis)
	Cost               int64   // modal unit, 0 = belum dibeli
	GradeFactor        float64 // faktor grade terpilih, dipakai hanya saat data grade kurang
	AccessoryDeduction int64   // potongan kelengkapan, dipakai hanya saat data grade kurang
	Weights            map[string]float64
	HalfLifeDays       float64
	Now                time.Time
}

type Result struct {
	Market      int64
	BuyLimit    int64
	Recommended int64
	MinPrice    int64
	Profit      int64
	P25, P75    int64
	Count       int
	LowData     bool
	Adjusted    bool
	Used        []Observation
	Outliers    []Observation
}

// Quote menghitung harga dari seluruh observasi model. Hanya seri jual yang dipakai.
func Quote(obs []Observation, grade string, p Params) Result {
	var sell []Observation
	for _, o := range obs {
		if o.Series == SeriesSell {
			sell = append(sell, o)
		}
	}

	base := filterGrade(sell, grade)
	res := Result{}
	if len(base) < 5 && len(sell) > len(base) {
		base = sell
		res.Adjusted = true
	}
	if p.HalfLifeDays <= 0 {
		p.HalfLifeDays = 30
	}
	if p.Now.IsZero() {
		p.Now = time.Now()
	}

	used, outliers := removeOutliers(base, p)
	for i := range used {
		used[i].Weight = weight(used[i], p)
	}
	res.Used = used
	res.Outliers = outliers
	res.Count = len(used)
	res.LowData = res.Count < 5
	if res.Count == 0 {
		return res
	}

	sort.SliceStable(used, func(i, j int) bool { return used[i].Price < used[j].Price })
	res.Market = weightedPercentile(used, 50)
	res.P25 = weightedPercentile(used, 25)
	res.P75 = weightedPercentile(used, 75)

	if res.Adjusted {
		f := p.GradeFactor
		if f <= 0 {
			f = 1
		}
		milli := int64(math.Round(f * 10000))
		res.Market = res.Market*milli/10000 - p.AccessoryDeduction
		if res.Market < 0 {
			res.Market = 0
		}
		res.P25 = res.P25 * milli / 10000
		res.P75 = res.P75 * milli / 10000
	}

	res.BuyLimit = res.Market - res.Market*int64(math.Round(p.TargetMargin*10000))/10000 - p.EstCost
	if res.BuyLimit < 0 {
		res.BuyLimit = 0
	}
	res.Recommended = roundUp10k(res.Market)
	if p.Cost > 0 {
		res.MinPrice = roundUp10k(p.Cost + p.Cost*int64(math.Round(p.MinMargin*10000))/10000)
		if res.MinPrice > res.Recommended {
			res.Recommended = res.MinPrice
		}
		res.Profit = res.Recommended - p.Cost
	}
	return res
}

func filterGrade(obs []Observation, grade string) []Observation {
	var out []Observation
	for _, o := range obs {
		if o.Grade == grade {
			out = append(out, o)
		}
	}
	return out
}

func weight(o Observation, p Params) float64 {
	w, ok := p.Weights[o.Source]
	if !ok || w <= 0 {
		w = 1
	}
	days := p.Now.Sub(o.ObservedAt).Hours() / 24
	if days < 0 {
		days = 0
	}
	return w * math.Pow(0.5, days/p.HalfLifeDays)
}

// removeOutliers membuang harga di luar Q1-1.5*IQR .. Q3+1.5*IQR.
func removeOutliers(obs []Observation, p Params) (used, outliers []Observation) {
	if len(obs) < 4 {
		return append([]Observation(nil), obs...), nil
	}
	prices := make([]float64, len(obs))
	for i, o := range obs {
		prices[i] = float64(o.Price)
	}
	sort.Float64s(prices)
	q1 := percentileSorted(prices, 25)
	q3 := percentileSorted(prices, 75)
	iqr := q3 - q1
	lo := q1 - 1.5*iqr
	hi := q3 + 1.5*iqr
	for _, o := range obs {
		if float64(o.Price) < lo || float64(o.Price) > hi {
			outliers = append(outliers, o)
			continue
		}
		used = append(used, o)
	}
	return used, outliers
}

func percentileSorted(sorted []float64, pct float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := (pct / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// weightedPercentile: used harus urut naik dan sudah punya Weight.
func weightedPercentile(used []Observation, pct float64) int64 {
	var total float64
	for _, o := range used {
		total += o.Weight
	}
	if total <= 0 {
		return 0
	}
	target := total * pct / 100
	var cum float64
	for _, o := range used {
		cum += o.Weight
		if cum >= target {
			return o.Price
		}
	}
	return used[len(used)-1].Price
}

func roundUp10k(v int64) int64 {
	if v <= 0 {
		return 0
	}
	return ((v + 9999) / 10000) * 10000
}

// WeightOf menghitung bobot satu observasi untuk ditampilkan.
func WeightOf(o Observation, p Params) float64 {
	if p.HalfLifeDays <= 0 {
		p.HalfLifeDays = 30
	}
	if p.Now.IsZero() {
		p.Now = time.Now()
	}
	return weight(o, p)
}
