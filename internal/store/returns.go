package store

import (
	"database/sql"
	"errors"
)

var ErrSaleVoided = errors.New("penjualan sudah dibatalkan")

// VoidSale membatalkan penjualan: unit kembali siap jual, kas dicatat sebagai
// pengeluaran pada sesi kas yang sedang buka, dan alasan disimpan untuk audit.
func (s *Store) VoidSale(saleID int64, by, reason string) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var unitID, branchID, total int64
	var invoice, method string
	var voidedAt sql.NullTime
	err = tx.QueryRow(`SELECT unit_id, branch_id, invoice, method, price - discount, voided_at
		FROM sales WHERE id = ? FOR UPDATE`, saleID).
		Scan(&unitID, &branchID, &invoice, &method, &total, &voidedAt)
	if err == sql.ErrNoRows {
		return 0, ErrUnitNotFound
	}
	if err != nil {
		return 0, err
	}
	if voidedAt.Valid {
		return 0, ErrSaleVoided
	}

	if _, err := tx.Exec(`UPDATE sales SET voided_at = NOW(), voided_by = ?, void_reason = ? WHERE id = ?`,
		by, reason, saleID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`UPDATE units SET status = 'siap', sold_at = NULL WHERE id = ?`, unitID); err != nil {
		return 0, err
	}
	// Harga dari penjualan yang batal tidak boleh dipakai sebagai data pasar.
	if _, err := tx.Exec(`DELETE FROM price_observations WHERE series = 'sell' AND source = 'toko' AND note = ?`,
		"penjualan "+invoice); err != nil {
		return 0, err
	}

	var sessionID sql.NullInt64
	_ = tx.QueryRow(`SELECT id FROM cash_sessions WHERE branch_id = ? AND status = 'buka' ORDER BY id DESC LIMIT 1`, branchID).Scan(&sessionID)
	if _, err := tx.Exec(`INSERT INTO cash_movements (session_id, branch_id, kind, method, amount, note, created_by, created_at)
		VALUES (?, ?, 'out', ?, ?, ?, ?, NOW())`,
		sessionID, branchID, method, total, "batal "+invoice, by); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.Audit(by, "batal jual", "sale", saleID, invoice, reason)
	return unitID, nil
}
