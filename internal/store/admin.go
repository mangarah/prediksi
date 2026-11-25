package store

import (
	"database/sql"
	"fmt"
)

func (s *Store) Setting(k string) (string, error) {
	var v string
	err := s.DB.QueryRow(`SELECT v FROM settings WHERE k = ?`, k).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) SaveSetting(k, v, user string) error {
	old, err := s.Setting(k)
	if err != nil {
		return err
	}
	if old == v {
		return nil
	}
	_, err = s.DB.Exec(`INSERT INTO settings (k, v) VALUES (?, ?) ON DUPLICATE KEY UPDATE v = VALUES(v)`, k, v)
	if err != nil {
		return err
	}
	s.Audit(user, "ubah", "setting "+k, 0, old, v)
	return nil
}

type CategoryMargin struct {
	Name string
	Bp   int
}

func (s *Store) CategoryMargins() ([]CategoryMargin, error) {
	rows, err := s.DB.Query(`SELECT name, target_margin_bp FROM categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategoryMargin
	for rows.Next() {
		var c CategoryMargin
		if err := rows.Scan(&c.Name, &c.Bp); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) SaveCategoryMargin(name string, bp int, user string) error {
	var old int
	if err := s.DB.QueryRow(`SELECT target_margin_bp FROM categories WHERE name = ?`, name).Scan(&old); err != nil {
		return err
	}
	if old == bp {
		return nil
	}
	if _, err := s.DB.Exec(`UPDATE categories SET target_margin_bp = ? WHERE name = ?`, bp, name); err != nil {
		return err
	}
	s.Audit(user, "ubah", "margin "+name, 0, fmt.Sprint(old), fmt.Sprint(bp))
	return nil
}

type Branch struct {
	ID   int64
	Name string
}

func (s *Store) Branches() ([]Branch, error) {
	rows, err := s.DB.Query(`SELECT id, name FROM branches ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Branch
	for rows.Next() {
		var b Branch
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

type SaleDetail struct {
	ID         int64
	Invoice    string
	BranchID   int64
	Branch     string
	SoldAt     string
	UnitCode   string
	Model      string
	Variant    string
	Grade      string
	Serial     string
	Method     string
	Buyer      string
	Price      int64
	Discount   int64
	Total      int64
	Cost       int64
	Profit     int64
	Warranty   int
	VoidedAt   string
	VoidedBy   string
	VoidReason string
}

func (s *Store) SaleDetail(id int64) (SaleDetail, error) {
	var d SaleDetail
	err := s.DB.QueryRow(`
		SELECT sa.id, sa.invoice, sa.branch_id, br.name, DATE_FORMAT(sa.sold_at, '%d %b %Y %H:%i'),
		       u.code, m.name, v.name, u.grade, COALESCE(u.serial,''), sa.method,
		       COALESCE(sa.buyer_name,''), sa.price, sa.discount, sa.price - sa.discount,
		       u.cost, sa.profit, sa.warranty_days,
		       COALESCE(DATE_FORMAT(sa.voided_at, '%d %b %Y %H:%i'), ''), COALESCE(sa.voided_by,''), COALESCE(sa.void_reason,'')
		FROM sales sa
		JOIN units u ON u.id = sa.unit_id
		JOIN product_models m ON m.id = u.model_id
		JOIN variants v ON v.id = u.variant_id
		JOIN branches br ON br.id = sa.branch_id
		WHERE sa.id = ?`, id).
		Scan(&d.ID, &d.Invoice, &d.BranchID, &d.Branch, &d.SoldAt, &d.UnitCode, &d.Model, &d.Variant,
			&d.Grade, &d.Serial, &d.Method, &d.Buyer, &d.Price, &d.Discount, &d.Total, &d.Cost, &d.Profit, &d.Warranty,
			&d.VoidedAt, &d.VoidedBy, &d.VoidReason)
	if err == sql.ErrNoRows {
		return d, ErrUnitNotFound
	}
	return d, err
}
