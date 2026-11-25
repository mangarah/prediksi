package store

import (
	"database/sql"
	"errors"
	"strconv"
)

type UnitPurchase struct {
	Seller  string
	Phone   string
	Date    string
	Deal    int64
	EstCost int64
	Status  string
}

type UnitSale struct {
	ID         int64
	Invoice    string
	Date       string
	Buyer      string
	Method     string
	Total      int64
	Profit     int64
	Voided     bool
	VoidReason string
}

type UnitTransfer struct {
	From       string
	To         string
	SentBy     string
	ReceivedBy string
	Date       string
	Status     string
}

type UnitDetail struct {
	Unit      Unit
	Purchase  UnitPurchase
	Sale      UnitSale
	HasSale   bool
	Transfers []UnitTransfer
	Audit     []AuditRow
	Suggested int64
}

func (s *Store) UnitByCode(code string) (Unit, error) {
	rows, err := s.DB.Query(unitSelect+` WHERE u.code = ?`, code)
	if err != nil {
		return Unit{}, err
	}
	us, err := scanUnits(rows)
	if err != nil {
		return Unit{}, err
	}
	if len(us) == 0 {
		return Unit{}, ErrUnitNotFound
	}
	return us[0], nil
}

func (s *Store) UnitDetail(id int64) (UnitDetail, error) {
	u, err := s.UnitByID(id)
	if err != nil {
		return UnitDetail{}, err
	}
	d := UnitDetail{Unit: u}

	var seller, phone, pdate, pstatus sql.NullString
	var deal, estCost sql.NullInt64
	_ = s.DB.QueryRow(`SELECT seller_name, COALESCE(seller_phone,''), DATE_FORMAT(created_at,'%d %b %Y'),
		deal_price, est_cost, status
		FROM purchases WHERE id = (SELECT purchase_id FROM units WHERE id = ?)`, id).
		Scan(&seller, &phone, &pdate, &deal, &estCost, &pstatus)
	d.Purchase = UnitPurchase{Seller: seller.String, Phone: phone.String, Date: pdate.String,
		Deal: deal.Int64, EstCost: estCost.Int64, Status: pstatus.String}

	var voided sql.NullTime
	err = s.DB.QueryRow(`SELECT id, invoice, DATE_FORMAT(sold_at,'%d %b %Y %H:%i'), COALESCE(buyer_name,''),
		method, price - discount, profit, voided_at, COALESCE(void_reason,'')
		FROM sales WHERE unit_id = ? ORDER BY id DESC LIMIT 1`, id).
		Scan(&d.Sale.ID, &d.Sale.Invoice, &d.Sale.Date, &d.Sale.Buyer, &d.Sale.Method,
			&d.Sale.Total, &d.Sale.Profit, &voided, &d.Sale.VoidReason)
	if err == nil {
		d.HasSale = true
		d.Sale.Voided = voided.Valid
	} else if err != sql.ErrNoRows {
		return d, err
	}

	rows, err := s.DB.Query(`SELECT bf.name, bt.name, t.sent_by, COALESCE(t.received_by,''),
		DATE_FORMAT(t.sent_at,'%d %b %Y %H:%i'), t.status
		FROM transfers t JOIN branches bf ON bf.id = t.from_branch JOIN branches bt ON bt.id = t.to_branch
		WHERE t.unit_id = ? ORDER BY t.id DESC`, id)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var t UnitTransfer
		if err := rows.Scan(&t.From, &t.To, &t.SentBy, &t.ReceivedBy, &t.Date, &t.Status); err != nil {
			rows.Close()
			return d, err
		}
		d.Transfers = append(d.Transfers, t)
	}
	rows.Close()

	if audit, err := s.AuditFor("unit", id, 20); err == nil {
		d.Audit = audit
	}
	if u.Status == "siap" && u.AgeDays > 30 {
		d.Suggested = SuggestPrice(u.ListedPrice, u.MinPrice)
	}
	return d, nil
}

// SuggestPrice menurunkan harga 5% dibulatkan ke bawah per Rp10.000, tidak di
// bawah harga minimal unit.
func SuggestPrice(listed, min int64) int64 {
	p := listed * 95 / 100
	p = p / 10000 * 10000
	if p < min {
		p = min
	}
	if p <= 0 {
		p = min
	}
	return p
}

func (s *Store) SetUnitPrice(id, price int64, by string) error {
	if price <= 0 {
		return errors.New("harga tidak valid")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old, min int64
	var status string
	if err := tx.QueryRow(`SELECT listed_price, min_price, status FROM units WHERE id = ? FOR UPDATE`, id).
		Scan(&old, &min, &status); err != nil {
		if err == sql.ErrNoRows {
			return ErrUnitNotFound
		}
		return err
	}
	if status == "terjual" || status == "kirim" {
		return ErrUnitNotReady
	}
	if price < min {
		return ErrBelowMin
	}
	if _, err := tx.Exec(`UPDATE units SET listed_price = ? WHERE id = ?`, price, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.Audit(by, "ubah harga", "unit", id, strconv.FormatInt(old, 10), strconv.FormatInt(price, 10))
	return nil
}
