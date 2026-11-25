package store

import (
	"database/sql"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists    = errors.New("email sudah dipakai")
	ErrWeakPassword  = errors.New("password minimal 8 karakter")
	ErrWrongPassword = errors.New("password lama salah")
)

type UserRow struct {
	ID       int64
	Name     string
	Email    string
	Role     string
	BranchID int64
	Branch   string
	Active   bool
}

func ValidRole(role string) bool {
	switch role {
	case "owner", "manajer", "buyer", "kasir":
		return true
	}
	return false
}

func ValidPassword(p string) bool { return len(strings.TrimSpace(p)) >= 8 }

// ValidBranchForRole menjaga non-owner tidak punya akses semua cabang (branch 0).
func ValidBranchForRole(role string, branchID int64) bool {
	return role == "owner" || branchID != 0
}

func (s *Store) Users() ([]UserRow, error) {
	rows, err := s.DB.Query(`SELECT u.id, u.name, u.email, u.role, COALESCE(u.branch_id,0),
		COALESCE(b.name,'Semua cabang'), u.active
		FROM users u LEFT JOIN branches b ON b.id = u.branch_id ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserRow
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.BranchID, &u.Branch, &u.Active); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) emailTaken(email string, exceptID int64) (bool, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ? AND id <> ?`, email, exceptID).Scan(&n)
	return n > 0, err
}

func (s *Store) CreateUser(name, email, password, role string, branchID int64) error {
	if !ValidRole(role) {
		return errors.New("peran tidak dikenal")
	}
	if !ValidPassword(password) {
		return ErrWeakPassword
	}
	if !ValidBranchForRole(role, branchID) {
		return errors.New("cabang wajib dipilih untuk peran ini")
	}
	taken, err := s.emailTaken(email, 0)
	if err != nil {
		return err
	}
	if taken {
		return ErrUserExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO users (name, email, password_hash, role, branch_id, active, created_at)
		VALUES (?, ?, ?, ?, NULLIF(?, 0), 1, NOW())`, name, email, string(hash), role, branchID)
	return err
}

func (s *Store) UpdateUser(id int64, name, email, role string, branchID int64, active bool) error {
	if !ValidRole(role) {
		return errors.New("peran tidak dikenal")
	}
	if !ValidBranchForRole(role, branchID) {
		return errors.New("cabang wajib dipilih untuk peran ini")
	}
	taken, err := s.emailTaken(email, id)
	if err != nil {
		return err
	}
	if taken {
		return ErrUserExists
	}
	_, err = s.DB.Exec(`UPDATE users SET name = ?, email = ?, role = ?, branch_id = NULLIF(?, 0), active = ? WHERE id = ?`,
		name, email, role, branchID, active, id)
	return err
}

func (s *Store) SetUserPassword(id int64, password string) error {
	if !ValidPassword(password) {
		return ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	return err
}

func (s *Store) ChangePassword(id int64, current, next string) error {
	if !ValidPassword(next) {
		return ErrWeakPassword
	}
	var hash string
	if err := s.DB.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, id).Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			return ErrUnitNotFound
		}
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		return ErrWrongPassword
	}
	return s.SetUserPassword(id, next)
}

// DeleteOtherSessions memaksa perangkat lain login ulang setelah password berubah.
func (s *Store) DeleteOtherSessions(userID int64, keepToken string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE user_id = ? AND token <> ?`, userID, keepToken)
	return err
}

func (s *Store) AddBranch(name string) error {
	_, err := s.DB.Exec(`INSERT INTO branches (name) VALUES (?)`, strings.TrimSpace(name))
	return err
}
