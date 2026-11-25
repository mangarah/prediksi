package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Transfer struct {
	ID         int64
	UnitID     int64
	UnitCode   string
	Model      string
	FromID     int64
	ToID       int64
	From       string
	To         string
	SentBy     string
	ReceivedBy string
	SentAt     time.Time
	ReceivedAt time.Time
	Status     string
}

func (s *Store) CreateTransfer(unitID, toBranch int64, by string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	var from int64
	if err := tx.QueryRow(`SELECT status, branch_id FROM units WHERE id = ? FOR UPDATE`, unitID).Scan(&status, &from); err != nil {
		return err
	}
	if status != "siap" {
		return errors.New("hanya unit siap jual yang bisa dikirim")
	}
	if from == toBranch {
		return errors.New("cabang tujuan sama dengan asal")
	}
	if _, err := tx.Exec(`INSERT INTO transfers (unit_id, from_branch, to_branch, sent_by, sent_at, status)
		VALUES (?, ?, ?, ?, NOW(), 'kirim')`, unitID, from, toBranch, by); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE units SET status = 'kirim' WHERE id = ?`, unitID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReceiveTransfer(id int64, by string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var unitID, to int64
	var status string
	if err := tx.QueryRow(`SELECT unit_id, to_branch, status FROM transfers WHERE id = ? FOR UPDATE`, id).Scan(&unitID, &to, &status); err != nil {
		return err
	}
	if status != "kirim" {
		return errors.New("transfer sudah diterima")
	}
	if _, err := tx.Exec(`UPDATE transfers SET status = 'terima', received_by = ?, received_at = NOW() WHERE id = ?`, by, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE units SET branch_id = ?, status = 'siap' WHERE id = ?`, to, unitID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) TransferByID(id int64) (Transfer, error) {
	var t Transfer
	err := s.DB.QueryRow(`
		SELECT t.id, t.unit_id, u.code, m.name, t.from_branch, t.to_branch, bf.name, bt.name,
		       t.sent_by, COALESCE(t.received_by,''), t.sent_at, COALESCE(t.received_at, t.sent_at), t.status
		FROM transfers t
		JOIN units u ON u.id = t.unit_id
		JOIN product_models m ON m.id = u.model_id
		JOIN branches bf ON bf.id = t.from_branch
		JOIN branches bt ON bt.id = t.to_branch
		WHERE t.id = ?`, id).
		Scan(&t.ID, &t.UnitID, &t.UnitCode, &t.Model, &t.FromID, &t.ToID, &t.From, &t.To,
			&t.SentBy, &t.ReceivedBy, &t.SentAt, &t.ReceivedAt, &t.Status)
	return t, err
}

func (s *Store) Transfers(branch int64) ([]Transfer, error) {
	rows, err := s.DB.Query(`
		SELECT t.id, t.unit_id, u.code, m.name, t.from_branch, t.to_branch, bf.name, bt.name,
		       t.sent_by, COALESCE(t.received_by,''), t.sent_at, COALESCE(t.received_at, t.sent_at), t.status
		FROM transfers t
		JOIN units u ON u.id = t.unit_id
		JOIN product_models m ON m.id = u.model_id
		JOIN branches bf ON bf.id = t.from_branch
		JOIN branches bt ON bt.id = t.to_branch
		WHERE (? = 0 OR t.from_branch = ? OR t.to_branch = ?)
		ORDER BY t.id DESC LIMIT 30`, branch, branch, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transfer
	for rows.Next() {
		var t Transfer
		if err := rows.Scan(&t.ID, &t.UnitID, &t.UnitCode, &t.Model, &t.FromID, &t.ToID, &t.From, &t.To,
			&t.SentBy, &t.ReceivedBy, &t.SentAt, &t.ReceivedAt, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) SerialExists(serial string) (bool, error) {
	if serial == "" {
		return false, nil
	}
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM units WHERE serial = ?`, serial).Scan(&n)
	return n > 0, err
}

func (s *Store) CreatePriceApproval(unitID, branchID, price, min int64, requester, title string) error {
	_, err := s.DB.Exec(`INSERT INTO approvals
		(kind, ref_id, branch_id, title, requester, reason, amount, compare_amount, status, created_at)
		VALUES ('price', ?, ?, ?, ?, 'Jual di bawah harga minimal', ?, ?, 'menunggu', NOW())`,
		unitID, branchID, title, requester, price, min)
	return err
}

// ---------- Impor stok ----------

type ImportUnit struct {
	Model, Variant, Grade, Serial string
	Cost, Listed, Min             int64
	Branch                        string
}

func (s *Store) ImportUnits(rows []ImportUnit) (int, []string, error) {
	models, err := s.Models()
	if err != nil {
		return 0, nil, err
	}
	branches, err := s.Branches()
	if err != nil {
		return 0, nil, err
	}
	type key struct{ model, variant string }
	pairs := map[key]Model{}
	for _, m := range models {
		pairs[key{m.Name, m.Variant}] = m
	}
	branchIDs := map[string]int64{}
	for _, b := range branches {
		branchIDs[strings.ToLower(b.Name)] = b.ID
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback()

	var errs []string
	inserted := 0
	for i, r := range rows {
		pair, ok := pairs[key{r.Model, r.Variant}]
		if !ok {
			errs = append(errs, fmt.Sprintf("baris %d: model/varian tidak dikenal (%s / %s)", i+2, r.Model, r.Variant))
			continue
		}
		branchID := branchIDs[strings.ToLower(r.Branch)]
		if branchID == 0 {
			errs = append(errs, fmt.Sprintf("baris %d: cabang tidak dikenal (%s)", i+2, r.Branch))
			continue
		}
		if r.Cost <= 0 || r.Listed <= 0 {
			errs = append(errs, fmt.Sprintf("baris %d: modal dan harga jual wajib > 0", i+2))
			continue
		}
		min := r.Min
		if min <= 0 {
			min = r.Cost + r.Cost*5/100
		}
		res, err := tx.Exec(`INSERT INTO units
			(code, branch_id, model_id, variant_id, grade, accessories, serial, cost, listed_price, min_price, status, created_at)
			VALUES (?, ?, ?, ?, ?, '[]', NULLIF(?, ''), ?, ?, ?, 'siap', NOW())`,
			fmt.Sprintf("%s-TMP%d", pair.Prefix, time.Now().UnixNano()), branchID, pair.ID, pair.VariantID,
			r.Grade, r.Serial, r.Cost, r.Listed, min)
		if err != nil {
			errs = append(errs, fmt.Sprintf("baris %d: %v", i+2, err))
			continue
		}
		id, _ := res.LastInsertId()
		if _, err := tx.Exec(`UPDATE units SET code = ? WHERE id = ?`, fmt.Sprintf("%s-%04d", pair.Prefix, id), id); err != nil {
			errs = append(errs, fmt.Sprintf("baris %d: %v", i+2, err))
			continue
		}
		inserted++
	}
	return inserted, errs, tx.Commit()
}

// ---------- Opname (hitung fisik) ----------

type OpnameResult struct {
	Expected, Found int
	Missing         []Unit
	Extra           []string
}

func (s *Store) Opname(branch int64, codes []string) (OpnameResult, error) {
	units, err := s.ReadyUnits(branch)
	if err != nil {
		return OpnameResult{}, err
	}
	found := map[string]bool{}
	for _, c := range codes {
		c = strings.TrimSpace(strings.ToUpper(c))
		if c != "" {
			found[c] = true
		}
	}
	res := OpnameResult{Expected: len(units)}
	known := map[string]bool{}
	for _, u := range units {
		known[strings.ToUpper(u.Code)] = true
		if found[strings.ToUpper(u.Code)] {
			res.Found++
		} else {
			res.Missing = append(res.Missing, u)
		}
	}
	for _, c := range codes {
		c = strings.TrimSpace(strings.ToUpper(c))
		if c != "" && !known[c] {
			res.Extra = append(res.Extra, c)
		}
	}
	return res, nil
}

var _ = sql.ErrNoRows
