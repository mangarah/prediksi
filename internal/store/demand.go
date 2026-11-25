package store

import (
	"math"
	"sort"
)

type DemandRow struct {
	ModelID    int64
	VariantID  int64
	Model      string
	Variant    string
	Category   string
	Sold30     int
	Stock      int
	MedianDays float64
	SpeedPct   int
	VolumePct  int
	Score      int
	Label      string
	Weekly     float64
	Need       int
}

type demandKey struct {
	model, variant int64
}

// DemandRows menghitung skor permintaan dari kecepatan laku dan volume 30 hari.
// Dua komponen (toko) dinormalkan ke bobot PRD 35% kecepatan + 25% volume.
func (s *Store) DemandRows(days int, branch int64) ([]DemandRow, error) {
	if days <= 0 {
		days = 30
	}
	models, err := s.Models()
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`
		SELECT u.model_id, u.variant_id, DATEDIFF(sa.sold_at, u.created_at)
		FROM sales sa JOIN units u ON u.id = sa.unit_id
		WHERE sa.voided_at IS NULL AND sa.sold_at >= DATE_SUB(NOW(), INTERVAL ? DAY) AND (? = 0 OR sa.branch_id = ?)`, days, branch, branch)
	if err != nil {
		return nil, err
	}
	sold := map[demandKey][]float64{}
	for rows.Next() {
		var m, v int64
		var d int
		if err := rows.Scan(&m, &v, &d); err != nil {
			rows.Close()
			return nil, err
		}
		if d < 0 {
			d = 0
		}
		k := demandKey{m, v}
		sold[k] = append(sold[k], float64(d))
	}
	rows.Close()

	rows, err = s.DB.Query(`SELECT model_id, variant_id, COUNT(*) FROM units
		WHERE status = 'siap' AND (? = 0 OR branch_id = ?) GROUP BY model_id, variant_id`, branch, branch)
	if err != nil {
		return nil, err
	}
	stock := map[demandKey]int{}
	for rows.Next() {
		var m, v int64
		var n int
		if err := rows.Scan(&m, &v, &n); err != nil {
			rows.Close()
			return nil, err
		}
		stock[demandKey{m, v}] = n
	}
	rows.Close()

	var out []DemandRow
	var speedVals, volumeVals []float64
	for _, m := range models {
		k := demandKey{m.ID, m.VariantID}
		ds := sold[k]
		r := DemandRow{ModelID: m.ID, VariantID: m.VariantID, Model: m.Name, Variant: m.Variant,
			Category: m.Category, Sold30: len(ds), Stock: stock[k]}
		if len(ds) > 0 {
			sort.Float64s(ds)
			r.MedianDays = median(ds)
			r.Weekly = float64(len(ds)) * 7 / float64(days)
		} else {
			r.MedianDays = float64(days)
		}
		need := int(math.Ceil(r.Weekly*2)) - r.Stock
		if need < 0 {
			need = 0
		}
		r.Need = need
		speedVals = append(speedVals, -r.MedianDays)
		volumeVals = append(volumeVals, float64(r.Sold30))
		out = append(out, r)
	}

	n := len(out)
	for i := range out {
		out[i].SpeedPct = pctRank(speedVals, speedVals[i], n)
		out[i].VolumePct = pctRank(volumeVals, volumeVals[i], n)
		if out[i].Sold30 == 0 {
			out[i].SpeedPct, out[i].VolumePct = 0, 0
		}
		out[i].Score = (out[i].SpeedPct*35 + out[i].VolumePct*25) / 60
		switch {
		case out[i].Score >= 70:
			out[i].Label = "Diminati tinggi"
		case out[i].Score >= 40:
			out[i].Label = "Normal"
		default:
			out[i].Label = "Lambat"
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func pctRank(vals []float64, v float64, n int) int {
	if n == 0 {
		return 0
	}
	le := 0
	for _, x := range vals {
		if x <= v {
			le++
		}
	}
	return le * 100 / n
}
