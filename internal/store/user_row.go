package store

func (s *Store) UserRowByID(id int64) (UserRow, error) {
	var u UserRow
	err := s.DB.QueryRow(`SELECT u.id, u.name, u.email, u.role, COALESCE(u.branch_id,0),
		COALESCE(b.name,'Semua cabang'), u.active
		FROM users u LEFT JOIN branches b ON b.id = u.branch_id WHERE u.id = ?`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.BranchID, &u.Branch, &u.Active)
	return u, err
}

func (s *Store) ActiveOwnerCount() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'owner' AND active = 1`).Scan(&n)
	return n, err
}
