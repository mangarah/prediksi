package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type TradeInInput struct {
	BranchID      int64
	OutUnitID     int64
	SalePrice     int64
	Discount      int64
	Method        string
	Buyer         string
	By            string
	InModelID     int64
	InVariantID   int64
	InPrefix      string
	InTitle       string
	InGrade       string
	InSerial      string
	InAccessories []string
	InValue       int64
	EstCost       int64
	Recommended   int64
	MinPrice      int64
	BuyLimit      int64
	Requester     string
}

// CreateTradeIn mencatat dua sisi sekaligus: unit masuk (nilai tukar) dan unit keluar (harga penuh).
func (s *Store) CreateTradeIn(in TradeInInput) (saleID, inUnitID, diff int64, needsApproval bool, err error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	var status, grade string
	var cost, minPrice, branch, modelID, variantID int64
	err = tx.QueryRow(`SELECT status, cost, min_price, branch_id, model_id, variant_id, grade
		FROM units WHERE id = ? FOR UPDATE`, in.OutUnitID).
		Scan(&status, &cost, &minPrice, &branch, &modelID, &variantID, &grade)
	if err != nil {
		return
	}
	if status != "siap" {
		err = ErrUnitNotReady
		return
	}
	if in.SalePrice < minPrice {
		err = ErrBelowMin
		return
	}
	if in.Method == "" {
		in.Method = "tunai"
	}

	needsApproval = in.InValue > in.BuyLimit || in.BuyLimit <= 0
	statusIn := "disetujui"
	unitStatus := "siap"
	if needsApproval {
		statusIn = "menunggu"
		unitStatus = "menunggu"
	}
	acc, _ := json.Marshal(in.InAccessories)

	res, err := tx.Exec(`INSERT INTO purchases
		(branch_id, model_id, variant_id, grade, seller_name, seller_phone, deal_price, est_cost, status, created_at)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, NOW())`,
		in.BranchID, in.InModelID, in.InVariantID, in.InGrade, in.Buyer, in.InValue, in.EstCost, statusIn)
	if err != nil {
		return
	}
	purchaseID, _ := res.LastInsertId()

	unitCost := in.InValue + in.EstCost
	res, err = tx.Exec(`INSERT INTO units
		(code, purchase_id, branch_id, model_id, variant_id, grade, accessories, serial, cost, listed_price, min_price, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, NOW())`,
		fmt.Sprintf("%s-TMP%d", in.InPrefix, time.Now().UnixNano()), purchaseID, in.BranchID, in.InModelID, in.InVariantID,
		in.InGrade, string(acc), in.InSerial, unitCost, in.Recommended, in.MinPrice, unitStatus)
	if err != nil {
		return
	}
	inUnitID, _ = res.LastInsertId()
	if _, err = tx.Exec(`UPDATE units SET code = ? WHERE id = ?`, fmt.Sprintf("%s-%04d", in.InPrefix, inUnitID), inUnitID); err != nil {
		return
	}
	if _, err = tx.Exec(`INSERT INTO price_observations
		(model_id, variant_id, grade, series, source, price, accessories, observed_at, note)
		VALUES (?, ?, ?, 'buy', 'penjual', ?, ?, NOW(), 'tukar tambah')`,
		in.InModelID, in.InVariantID, in.InGrade, in.InValue, string(acc)); err != nil {
		return
	}
	if needsApproval {
		reason := fmt.Sprintf("Nilai tukar %d di atas batas beli %d", in.InValue, in.BuyLimit)
		if _, err = tx.Exec(`INSERT INTO approvals
			(kind, ref_id, branch_id, title, requester, reason, amount, compare_amount, status, created_at)
			VALUES ('purchase', ?, ?, ?, ?, ?, ?, ?, 'menunggu', NOW())`,
			purchaseID, in.BranchID, in.InTitle, in.Requester, reason, in.InValue, in.BuyLimit); err != nil {
			return
		}
	}

	profit := in.SalePrice - in.Discount - cost
	res, err = tx.Exec(`INSERT INTO sales
		(invoice, unit_id, branch_id, price, discount, method, buyer_name, sold_by, warranty_days, profit, sold_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 14, ?, NOW())`,
		fmt.Sprintf("TMP-%d", time.Now().UnixNano()), in.OutUnitID, branch, in.SalePrice, in.Discount, in.Method,
		in.Buyer, in.By, profit)
	if err != nil {
		return
	}
	saleID, _ = res.LastInsertId()
	invoice := fmt.Sprintf("INV-%05d", saleID)
	if _, err = tx.Exec(`UPDATE sales SET invoice = ? WHERE id = ?`, invoice, saleID); err != nil {
		return
	}
	if _, err = tx.Exec(`UPDATE units SET status = 'terjual', sold_at = NOW() WHERE id = ?`, in.OutUnitID); err != nil {
		return
	}
	if _, err = tx.Exec(`INSERT INTO price_observations
		(model_id, variant_id, grade, series, source, price, accessories, observed_at, note)
		VALUES (?, ?, ?, 'sell', 'toko', ?, NULL, NOW(), ?)`,
		modelID, variantID, grade, in.SalePrice-in.Discount, "tukar tambah "+invoice); err != nil {
		return
	}

	diff = in.SalePrice - in.Discount - in.InValue
	if diff != 0 {
		kind := "in"
		amount := diff
		if diff < 0 {
			kind, amount = "out", -diff
		}
		var sessionID sql.NullInt64
		_ = tx.QueryRow(`SELECT id FROM cash_sessions WHERE branch_id = ? AND status = 'buka' ORDER BY id DESC LIMIT 1`, branch).Scan(&sessionID)
		if _, err = tx.Exec(`INSERT INTO cash_movements
			(session_id, branch_id, kind, method, amount, note, created_by, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`,
			sessionID, branch, kind, in.Method, amount, "tukar tambah "+invoice, in.By); err != nil {
			return
		}
	}
	return saleID, inUnitID, diff, needsApproval, tx.Commit()
}
