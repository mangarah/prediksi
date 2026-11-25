package store

import (
	"fmt"
	"time"
)

type ReportDay struct {
	Label   string
	Units   int
	Revenue int64
	Profit  int64
}

type ReportGroup struct {
	Name    string
	Units   int
	Revenue int64
	Profit  int64
}

type Report struct {
	From       string
	To         string
	Units      int
	Revenue    int64
	Profit     int64
	MarginPct  int
	Days       []ReportDay
	Cats       []ReportGroup
	Staff      []ReportGroup
	StockValue int64
	Stale      int
}

func (s *Store) Report(from, to time.Time, branch int64) (Report, error) {
	r := Report{From: from.Format("2 Jan 2006"), To: to.Format("2 Jan 2006")}
	start := from.Format("2006-01-02") + " 00:00:00"
	end := to.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00:00"

	if err := s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(price-discount),0), COALESCE(SUM(profit),0)
		FROM sales WHERE voided_at IS NULL AND sold_at >= ? AND sold_at < ? AND (? = 0 OR branch_id = ?)`, start, end, branch, branch).
		Scan(&r.Units, &r.Revenue, &r.Profit); err != nil {
		return r, err
	}
	if r.Revenue > 0 {
		r.MarginPct = int(r.Profit * 100 / r.Revenue)
	}

	rows, err := s.DB.Query(`SELECT DATE_FORMAT(sold_at,'%d %b'), COUNT(*), SUM(price-discount), SUM(profit)
		FROM sales WHERE voided_at IS NULL AND sold_at >= ? AND sold_at < ? AND (? = 0 OR branch_id = ?)
		GROUP BY DATE(sold_at), DATE_FORMAT(sold_at,'%d %b') ORDER BY DATE(sold_at)`, start, end, branch, branch)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var d ReportDay
		if err := rows.Scan(&d.Label, &d.Units, &d.Revenue, &d.Profit); err != nil {
			rows.Close()
			return r, err
		}
		r.Days = append(r.Days, d)
	}
	rows.Close()

	rows, err = s.DB.Query(`
		SELECT c.name, COUNT(*), SUM(sa.price-sa.discount), SUM(sa.profit)
		FROM sales sa
		JOIN units u ON u.id = sa.unit_id
		JOIN product_models m ON m.id = u.model_id
		JOIN brands b ON b.id = m.brand_id
		JOIN categories c ON c.id = b.category_id
		WHERE sa.voided_at IS NULL AND sa.sold_at >= ? AND sa.sold_at < ? AND (? = 0 OR sa.branch_id = ?)
		GROUP BY c.id, c.name ORDER BY SUM(sa.profit) DESC`, start, end, branch, branch)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var g ReportGroup
		if err := rows.Scan(&g.Name, &g.Units, &g.Revenue, &g.Profit); err != nil {
			rows.Close()
			return r, err
		}
		r.Cats = append(r.Cats, g)
	}
	rows.Close()

	rows, err = s.DB.Query(`
		SELECT COALESCE(NULLIF(sold_by,''),'Tanpa nama'), COUNT(*), SUM(price-discount), SUM(profit)
		FROM sales WHERE voided_at IS NULL AND sold_at >= ? AND sold_at < ? AND (? = 0 OR branch_id = ?)
		GROUP BY COALESCE(NULLIF(sold_by,''),'Tanpa nama') ORDER BY SUM(profit) DESC`, start, end, branch, branch)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var g ReportGroup
		if err := rows.Scan(&g.Name, &g.Units, &g.Revenue, &g.Profit); err != nil {
			rows.Close()
			return r, err
		}
		r.Staff = append(r.Staff, g)
	}
	rows.Close()

	if err := s.DB.QueryRow(`SELECT COALESCE(SUM(cost),0),
		COALESCE(SUM(status = 'siap' AND created_at < DATE_SUB(NOW(), INTERVAL 30 DAY)),0)
		FROM units WHERE status IN ('siap','servis','dicek','menunggu') AND (? = 0 OR branch_id = ?)`, branch, branch).
		Scan(&r.StockValue, &r.Stale); err != nil {
		return r, err
	}
	return r, nil
}

type SaleLine struct {
	Invoice  string
	Date     string
	Branch   string
	Model    string
	Variant  string
	Grade    string
	Serial   string
	Buyer    string
	SoldBy   string
	Method   string
	Price    int64
	Discount int64
	Total    int64
	Cost     int64
	Profit   int64
}

func (s *Store) SaleLines(from, to time.Time, branch int64) ([]SaleLine, error) {
	start := from.Format("2006-01-02") + " 00:00:00"
	end := to.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00:00"
	rows, err := s.DB.Query(`
		SELECT sa.invoice, DATE_FORMAT(sa.sold_at,'%Y-%m-%d %H:%i'), br.name, m.name, v.name, u.grade,
		       COALESCE(u.serial,''), COALESCE(sa.buyer_name,''), COALESCE(sa.sold_by,''), sa.method,
		       sa.price, sa.discount, sa.price-sa.discount, u.cost, sa.profit
		FROM sales sa
		JOIN units u ON u.id = sa.unit_id
		JOIN product_models m ON m.id = u.model_id
		JOIN variants v ON v.id = u.variant_id
		JOIN branches br ON br.id = sa.branch_id
		WHERE sa.voided_at IS NULL AND sa.sold_at >= ? AND sa.sold_at < ? AND (? = 0 OR sa.branch_id = ?)
		ORDER BY sa.sold_at`, start, end, branch, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SaleLine
	for rows.Next() {
		var l SaleLine
		if err := rows.Scan(&l.Invoice, &l.Date, &l.Branch, &l.Model, &l.Variant, &l.Grade, &l.Serial,
			&l.Buyer, &l.SoldBy, &l.Method, &l.Price, &l.Discount, &l.Total, &l.Cost, &l.Profit); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) LastSaleForUnit(unitID int64) (int64, error) {
	var id int64
	err := s.DB.QueryRow(`SELECT id FROM sales WHERE voided_at IS NULL AND unit_id = ? ORDER BY id DESC LIMIT 1`, unitID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("nota unit: %w", err)
	}
	return id, nil
}
