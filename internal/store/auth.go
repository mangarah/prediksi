package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNoCashSession = errors.New("belum ada sesi kas")

type User struct {
	ID       int64
	Name     string
	Email    string
	Role     string
	BranchID int64
	Branch   string
}

func (s *Store) UserByEmail(email string) (User, string, error) {
	var u User
	var hash string
	err := s.DB.QueryRow(`
		SELECT u.id, u.name, u.email, u.role, COALESCE(u.branch_id,0), COALESCE(b.name,'Semua cabang'), u.password_hash
		FROM users u LEFT JOIN branches b ON b.id = u.branch_id
		WHERE u.email = ? AND u.active = 1`, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.BranchID, &u.Branch, &hash)
	if err == sql.ErrNoRows {
		return User{}, "", sql.ErrNoRows
	}
	return u, hash, err
}

func (s *Store) UserBySession(token string) (User, error) {
	var u User
	err := s.DB.QueryRow(`
		SELECT u.id, u.name, u.email, u.role, COALESCE(u.branch_id,0), COALESCE(b.name,'Semua cabang')
		FROM sessions se
		JOIN users u ON u.id = se.user_id
		LEFT JOIN branches b ON b.id = u.branch_id
		WHERE se.token = ? AND se.expires_at > NOW() AND u.active = 1`, token).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.BranchID, &u.Branch)
	if err == sql.ErrNoRows {
		return User{}, sql.ErrNoRows
	}
	return u, err
}

func (s *Store) CreateSession(userID int64, token string, expires time.Time) error {
	_, err := s.DB.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expires)
	return err
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Store) AuditFor(entity string, entityID int64, limit int) ([]AuditRow, error) {
	rows, err := s.DB.Query(`SELECT user_name, action, entity, COALESCE(entity_id,0),
		COALESCE(old_value,''), COALESCE(new_value,''), created_at
		FROM audit_logs WHERE entity = ? AND entity_id = ? ORDER BY id DESC LIMIT ?`, entity, entityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.UserName, &a.Action, &a.Entity, &a.EntityID, &a.OldValue, &a.NewValue, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type AuditRow struct {
	UserName  string
	Action    string
	Entity    string
	EntityID  int64
	OldValue  string
	NewValue  string
	CreatedAt time.Time
}

func (s *Store) Audit(user, action, entity string, entityID int64, oldV, newV string) {
	_, err := s.DB.Exec(`INSERT INTO audit_logs (user_name, action, entity, entity_id, old_value, new_value, created_at)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NOW())`, user, action, entity, entityID, oldV, newV)
	if err != nil {
		// audit tidak boleh menggagalkan transaksi utama, tapi harus kelihatan di log server
		fmt.Printf("audit gagal: %v\n", err)
	}
}

func (s *Store) AuditRecent(limit int) ([]AuditRow, error) {
	rows, err := s.DB.Query(`SELECT user_name, action, entity, COALESCE(entity_id,0), COALESCE(old_value,''), COALESCE(new_value,''), created_at
		FROM audit_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.UserName, &a.Action, &a.Entity, &a.EntityID, &a.OldValue, &a.NewValue, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type NewObservation struct {
	ModelID     int64
	VariantID   int64
	Grade       string
	Series      string
	Source      string
	Price       int64
	Accessories []string
	ObservedAt  time.Time
	Note        string
}

func (s *Store) AddObservation(in NewObservation) (int64, error) {
	acc, _ := json.Marshal(in.Accessories)
	r, err := s.DB.Exec(`INSERT INTO price_observations
		(model_id, variant_id, grade, series, source, price, accessories, observed_at, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''))`,
		in.ModelID, in.VariantID, in.Grade, in.Series, in.Source, in.Price, string(acc), in.ObservedAt, in.Note)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

type ObsRow struct {
	ID         int64
	Model      string
	Variant    string
	Grade      string
	Series     string
	Source     string
	Price      int64
	ObservedAt time.Time
	Note       string
}

func (s *Store) RecentObservations(modelID int64, limit int) ([]ObsRow, error) {
	rows, err := s.DB.Query(`
		SELECT o.id, m.name, v.name, o.grade, o.series, o.source, o.price, o.observed_at, COALESCE(o.note,'')
		FROM price_observations o
		JOIN product_models m ON m.id = o.model_id
		JOIN variants v ON v.id = o.variant_id
		WHERE (? = 0 OR o.model_id = ?)
		ORDER BY o.observed_at DESC, o.id DESC LIMIT ?`, modelID, modelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObsRow
	for rows.Next() {
		var o ObsRow
		if err := rows.Scan(&o.ID, &o.Model, &o.Variant, &o.Grade, &o.Series, &o.Source, &o.Price, &o.ObservedAt, &o.Note); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

type CashSession struct {
	ID         int64
	BranchID   int64
	Branch     string
	OpenedBy   string
	Opening    int64
	Expected   int64
	Counted    int64
	Difference int64
	OpenedAt   time.Time
	ClosedAt   time.Time
	Status     string
}

func (s *Store) OpenCash(branchID int64, by string, opening int64) (int64, error) {
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM cash_sessions WHERE branch_id = ? AND status IN ('buka','menunggu')`, branchID).Scan(&n); err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, errors.New("kas cabang ini masih terbuka")
	}
	r, err := s.DB.Exec(`INSERT INTO cash_sessions (branch_id, opened_by, opening_balance, opened_at, status)
		VALUES (?, ?, ?, NOW(), 'buka')`, branchID, by, opening)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (s *Store) CurrentCash(branchID int64) (CashSession, error) {
	var c CashSession
	err := s.DB.QueryRow(`
		SELECT id, branch_id, opened_by, opening_balance, COALESCE(expected_balance,0), COALESCE(counted_balance,0),
		       COALESCE(difference,0), opened_at, COALESCE(closed_at, opened_at), status
		FROM cash_sessions
		WHERE branch_id = ? AND status IN ('buka','menunggu')
		ORDER BY id DESC LIMIT 1`, branchID).
		Scan(&c.ID, &c.BranchID, &c.OpenedBy, &c.Opening, &c.Expected, &c.Counted, &c.Difference, &c.OpenedAt, &c.ClosedAt, &c.Status)
	if err == sql.ErrNoRows {
		return c, ErrNoCashSession
	}
	return c, err
}

type CashMovement struct {
	ID        int64
	Kind      string
	Method    string
	Amount    int64
	Note      string
	CreatedBy string
	CreatedAt time.Time
}

func (s *Store) CashMovements(sessionID int64) ([]CashMovement, error) {
	rows, err := s.DB.Query(`
		SELECT id, kind, method, amount, COALESCE(note,''), created_by, created_at
		FROM cash_movements WHERE session_id = ? ORDER BY id DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CashMovement
	for rows.Next() {
		var m CashMovement
		if err := rows.Scan(&m.ID, &m.Kind, &m.Method, &m.Amount, &m.Note, &m.CreatedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AddCashMovement(sessionID, branchID int64, kind, method string, amount int64, note, by string) error {
	var sid sql.NullInt64
	if sessionID > 0 {
		sid = sql.NullInt64{Int64: sessionID, Valid: true}
	}
	_, err := s.DB.Exec(`INSERT INTO cash_movements (session_id, branch_id, kind, method, amount, note, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`, sid, branchID, kind, method, amount, note, by)
	return err
}

// CloseCash menghitung selisih kas. Di atas ambang, sesi menunggu persetujuan.
func (s *Store) CloseCash(sessionID int64, by string, counted, threshold int64) (expected, diff int64, needsApproval bool, err error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	var opening, branchID int64
	var status string
	if err = tx.QueryRow(`SELECT opening_balance, branch_id, status FROM cash_sessions WHERE id = ? FOR UPDATE`, sessionID).
		Scan(&opening, &branchID, &status); err != nil {
		return
	}
	if status != "buka" {
		err = errors.New("sesi kas tidak bisa ditutup")
		return
	}

	err = tx.QueryRow(`
		SELECT CAST(? +
		  COALESCE(SUM(CASE WHEN kind IN ('sale','in') AND method = 'tunai' THEN amount ELSE 0 END),0) -
		  COALESCE(SUM(CASE WHEN kind IN ('purchase','expense','out') AND method = 'tunai' THEN amount ELSE 0 END),0)
		  AS SIGNED)
		FROM cash_movements WHERE session_id = ?`, opening, sessionID).Scan(&expected)
	if err != nil {
		return
	}
	diff = counted - expected
	needsApproval = abs64(diff) > threshold
	newStatus := "tutup"
	if needsApproval {
		newStatus = "menunggu"
	}

	if _, err = tx.Exec(`UPDATE cash_sessions SET expected_balance = ?, counted_balance = ?, difference = ?, status = ?,
		closed_at = IF(? = 'tutup', NOW(), NULL) WHERE id = ?`, expected, counted, diff, newStatus, newStatus, sessionID); err != nil {
		return
	}
	if needsApproval {
		_, err = tx.Exec(`INSERT INTO approvals
			(kind, ref_id, branch_id, title, requester, reason, amount, compare_amount, status, created_at)
			VALUES ('cash', ?, ?, ?, ?, ?, ?, ?, 'menunggu', NOW())`,
			sessionID, branchID, "Tutup kas · selisih di atas ambang", by,
			"Selisih kas melebihi ambang", expected, counted)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
