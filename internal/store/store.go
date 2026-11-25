// Package store menyimpan seluruh akses database MySQL dan migrasi skema.
package store

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"prediksi/internal/pricing"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	ErrUnitNotReady    = errors.New("unit tidak siap dijual")
	ErrBelowMin        = errors.New("harga di bawah harga minimal")
	ErrUnitNotFound    = errors.New("unit tidak ditemukan")
	ErrDuplicateSerial = errors.New("nomor seri sudah pernah masuk")
)

type Store struct {
	DB *sql.DB
}

func Open(user, pass, addr, dbName string) (*Store, error) {
	adminCfg := mysql.NewConfig()
	adminCfg.User = user
	adminCfg.Passwd = pass
	adminCfg.Net = "tcp"
	adminCfg.Addr = addr
	adminCfg.ParseTime = true
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	defer admin.Close()
	if _, err := admin.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "` CHARACTER SET utf8mb4"); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	cfg := mysql.NewConfig()
	cfg.User = user
	cfg.Passwd = pass
	cfg.Net = "tcp"
	cfg.Addr = addr
	cfg.DBName = dbName
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Migrate() error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("mysql"); err != nil {
		return err
	}
	return goose.Up(s.DB, "migrations")
}

type Model struct {
	ID           int64
	VariantID    int64
	Name         string
	Variant      string
	Category     string
	Prefix       string
	TargetMargin float64
}

func (s *Store) Models() ([]Model, error) {
	rows, err := s.DB.Query(`
		SELECT m.id, v.id, m.name, v.name, c.name, m.prefix, c.target_margin_bp
		FROM product_models m
		JOIN brands b ON b.id = m.brand_id
		JOIN categories c ON c.id = b.category_id
		JOIN variants v ON v.model_id = m.id
		ORDER BY c.sort_order, m.id, v.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Model
	for rows.Next() {
		var m Model
		var bp int
		if err := rows.Scan(&m.ID, &m.VariantID, &m.Name, &m.Variant, &m.Category, &m.Prefix, &bp); err != nil {
			return nil, err
		}
		m.TargetMargin = float64(bp) / 10000
		out = append(out, m)
	}
	return out, rows.Err()
}

type Settings struct {
	TargetMargin       map[string]float64
	MinMargin          float64
	EstCost            int64
	PeriodDays         int
	HalfLifeDays       float64
	CashThreshold      int64
	Weights            map[string]float64
	GradeFactors       map[string]float64
	AccessoryDeduction map[string]int64
}

func (s *Store) Settings() (Settings, error) {
	st := Settings{
		TargetMargin:       map[string]float64{},
		MinMargin:          0.05,
		EstCost:            100_000,
		PeriodDays:         60,
		HalfLifeDays:       30,
		CashThreshold:      10_000,
		Weights:            map[string]float64{"toko": 1, "pesaing": 0.8, "marketplace": 0.6, "penjual": 1},
		GradeFactors:       map[string]float64{"A": 1, "B": 0.9, "C": 0.78, "D": 0.5},
		AccessoryDeduction: map[string]int64{},
	}

	rows, err := s.DB.Query(`SELECT k, v FROM settings`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return st, err
		}
		switch k {
		case "min_margin_bp":
			st.MinMargin = bp(v)
		case "est_cost":
			st.EstCost = atoi(v)
		case "period_days":
			st.PeriodDays = int(atoi(v))
		case "half_life_days":
			st.HalfLifeDays = float64(atoi(v))
		case "cash_threshold":
			st.CashThreshold = atoi(v)
		case "w_toko":
			st.Weights["toko"] = bp(v)
		case "w_pesaing":
			st.Weights["pesaing"] = bp(v)
		case "w_marketplace":
			st.Weights["marketplace"] = bp(v)
		case "w_penjual":
			st.Weights["penjual"] = bp(v)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}

	if rows, err = s.DB.Query(`SELECT name, target_margin_bp FROM categories`); err != nil {
		return st, err
	}
	for rows.Next() {
		var name string
		var v int
		if err := rows.Scan(&name, &v); err != nil {
			rows.Close()
			return st, err
		}
		st.TargetMargin[name] = float64(v) / 10000
	}
	rows.Close()

	if rows, err = s.DB.Query(`SELECT code, factor_bp FROM grades`); err != nil {
		return st, err
	}
	for rows.Next() {
		var code string
		var v int
		if err := rows.Scan(&code, &v); err != nil {
			rows.Close()
			return st, err
		}
		st.GradeFactors[code] = float64(v) / 10000
	}
	rows.Close()

	if rows, err = s.DB.Query(`SELECT code, deduction FROM accessories`); err != nil {
		return st, err
	}
	for rows.Next() {
		var code string
		var v int64
		if err := rows.Scan(&code, &v); err != nil {
			rows.Close()
			return st, err
		}
		st.AccessoryDeduction[code] = v
	}
	return st, rows.Err()
}

type Accessory struct {
	Code      string
	Label     string
	Deduction int64
}

func (s *Store) Accessories() ([]Accessory, error) {
	rows, err := s.DB.Query(`SELECT code, label, deduction FROM accessories ORDER BY deduction DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Accessory
	for rows.Next() {
		var a Accessory
		if err := rows.Scan(&a.Code, &a.Label, &a.Deduction); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) Observations(modelID, variantID int64, days int) ([]pricing.Observation, error) {
	if days <= 0 {
		days = 60
	}
	rows, err := s.DB.Query(`
		SELECT price, series, source, grade, observed_at, accessories
		FROM price_observations
		WHERE model_id = ? AND variant_id = ? AND observed_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		ORDER BY observed_at DESC`, modelID, variantID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pricing.Observation
	for rows.Next() {
		var o pricing.Observation
		var acc sql.NullString
		if err := rows.Scan(&o.Price, &o.Series, &o.Source, &o.Grade, &o.ObservedAt, &acc); err != nil {
			return nil, err
		}
		if acc.Valid && acc.String != "" {
			_ = json.Unmarshal([]byte(acc.String), &o.Accessories)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

type Unit struct {
	ID          int64
	Code        string
	Serial      string
	Model       string
	Variant     string
	Grade       string
	Category    string
	Branch      string
	BranchID    int64
	Status      string
	Cost        int64
	ListedPrice int64
	MinPrice    int64
	Photo       string
	CreatedAt   time.Time
	AgeDays     int
}

const unitSelect = `
	SELECT u.id, u.code, COALESCE(u.serial,''), m.name, v.name, u.grade, c.name, br.name, u.status,
	       u.cost, u.listed_price, u.min_price, u.created_at, u.branch_id, COALESCE(p.photo,'')
	FROM units u
	JOIN product_models m ON m.id = u.model_id
	JOIN variants v ON v.id = u.variant_id
	JOIN brands b ON b.id = m.brand_id
	JOIN categories c ON c.id = b.category_id
	JOIN branches br ON br.id = u.branch_id
	LEFT JOIN purchases p ON p.id = u.purchase_id`

func scanUnits(rows *sql.Rows) ([]Unit, error) {
	defer rows.Close()
	var out []Unit
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.Code, &u.Serial, &u.Model, &u.Variant, &u.Grade, &u.Category,
			&u.Branch, &u.Status, &u.Cost, &u.ListedPrice, &u.MinPrice, &u.CreatedAt, &u.BranchID, &u.Photo); err != nil {
			return nil, err
		}
		u.AgeDays = int(time.Since(u.CreatedAt).Hours() / 24)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) ReadyUnits(branch int64) ([]Unit, error) {
	rows, err := s.DB.Query(unitSelect+` WHERE u.status = 'siap' AND (? = 0 OR u.branch_id = ?) ORDER BY u.created_at DESC`, branch, branch)
	if err != nil {
		return nil, err
	}
	return scanUnits(rows)
}

func (s *Store) RecentUnits(limit int, branch int64) ([]Unit, error) {
	rows, err := s.DB.Query(unitSelect+` WHERE (? = 0 OR u.branch_id = ?) ORDER BY u.created_at DESC LIMIT ?`, branch, branch, limit)
	if err != nil {
		return nil, err
	}
	return scanUnits(rows)
}

func (s *Store) UnitsByStatus(status string, branch int64, limit int) ([]Unit, error) {
	if status == "" {
		status = "siap"
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.DB.Query(unitSelect+` WHERE u.status = ? AND (? = 0 OR u.branch_id = ?) ORDER BY u.created_at DESC LIMIT ?`,
		status, branch, branch, limit)
	if err != nil {
		return nil, err
	}
	return scanUnits(rows)
}

func (s *Store) UnitByID(id int64) (Unit, error) {
	rows, err := s.DB.Query(unitSelect+` WHERE u.id = ?`, id)
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

type KPIs struct {
	ProfitToday  int64
	RevenueToday int64
	CashToday    int64
	SalesToday   int
	ReadyUnits   int
	StaleUnits   int
	Pending      int
	StockValue   int64
}

func (s *Store) KPIs(branch int64) (KPIs, error) {
	var k KPIs
	err := s.DB.QueryRow(`
		SELECT COALESCE(SUM(profit),0), COALESCE(SUM(price - discount),0),
		       COALESCE(SUM(CASE WHEN method = 'tunai' THEN price - discount ELSE 0 END),0), COUNT(*)
		FROM sales WHERE voided_at IS NULL AND DATE(sold_at) = CURDATE() AND (? = 0 OR branch_id = ?)`, branch, branch).Scan(&k.ProfitToday, &k.RevenueToday, &k.CashToday, &k.SalesToday)
	if err != nil {
		return k, err
	}
	err = s.DB.QueryRow(`
		SELECT COALESCE(SUM(status = 'siap'),0),
		       COALESCE(SUM(status = 'siap' AND created_at < DATE_SUB(NOW(), INTERVAL 30 DAY)),0),
		       COALESCE(SUM(CASE WHEN status IN ('siap','servis','dicek','menunggu') THEN cost ELSE 0 END),0)
		FROM units WHERE (? = 0 OR branch_id = ?)`, branch, branch).Scan(&k.ReadyUnits, &k.StaleUnits, &k.StockValue)
	if err != nil {
		return k, err
	}
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM approvals WHERE status = 'menunggu' AND (? = 0 OR branch_id = ?)`, branch, branch).Scan(&k.Pending)
	return k, err
}

type DayPoint struct {
	Label  string
	Total  int64
	Height int
}

func (s *Store) Sales7Days(branch int64) ([]DayPoint, error) {
	rows, err := s.DB.Query(`
		SELECT DATE(sold_at) AS d, SUM(price - discount) AS total
		FROM sales
		WHERE voided_at IS NULL AND sold_at >= DATE_SUB(CURDATE(), INTERVAL 6 DAY) AND (? = 0 OR branch_id = ?)
		GROUP BY DATE(sold_at)`, branch, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	totals := map[string]int64{}
	var max int64
	for rows.Next() {
		var d time.Time
		var total int64
		if err := rows.Scan(&d, &total); err != nil {
			return nil, err
		}
		totals[d.Format("2006-01-02")] = total
		if total > max {
			max = total
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []DayPoint
	for i := 6; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i)
		key := d.Format("2006-01-02")
		p := DayPoint{Label: d.Format("2 Jan"), Total: totals[key]}
		if max > 0 {
			p.Height = int(totals[key] * 100 / max)
			if p.Height < 4 && totals[key] > 0 {
				p.Height = 4
			}
		}
		out = append(out, p)
	}
	return out, nil
}

type CatSlice struct {
	Name  string
	Units int
	Pct   int
	Color string
}

func (s *Store) SalesByCategory(days int, branch int64) ([]CatSlice, error) {
	rows, err := s.DB.Query(`
		SELECT c.name, COUNT(*)
		FROM sales sa
		JOIN units u ON u.id = sa.unit_id
		JOIN product_models m ON m.id = u.model_id
		JOIN brands b ON b.id = m.brand_id
		JOIN categories c ON c.id = b.category_id
		WHERE sa.voided_at IS NULL AND sa.sold_at >= DATE_SUB(NOW(), INTERVAL ? DAY) AND (? = 0 OR sa.branch_id = ?)
		GROUP BY c.id, c.name
		ORDER BY COUNT(*) DESC`, days, branch, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CatSlice
	var total int
	for rows.Next() {
		var c CatSlice
		if err := rows.Scan(&c.Name, &c.Units); err != nil {
			return nil, err
		}
		total += c.Units
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	colors := []string{"#0f766e", "#14b8a6", "#99f6e4", "#64748b", "#cbd5e1"}
	for i := range out {
		out[i].Color = colors[i%len(colors)]
		if total > 0 {
			out[i].Pct = out[i].Units * 100 / total
		}
	}
	return out, nil
}

type SaleRow struct {
	ID      int64
	Invoice string
	Unit    string
	Total   int64
	Method  string
	Time    string
}

func (s *Store) SalesToday(branch int64) ([]SaleRow, error) {
	rows, err := s.DB.Query(`
		SELECT sa.id, sa.invoice, u.code, sa.price - sa.discount, sa.method, DATE_FORMAT(sa.sold_at, '%H:%i')
		FROM sales sa JOIN units u ON u.id = sa.unit_id
		WHERE sa.voided_at IS NULL AND DATE(sa.sold_at) = CURDATE() AND (? = 0 OR sa.branch_id = ?)
		ORDER BY sa.sold_at DESC`, branch, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SaleRow
	for rows.Next() {
		var r SaleRow
		if err := rows.Scan(&r.ID, &r.Invoice, &r.Unit, &r.Total, &r.Method, &r.Time); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Approval struct {
	ID        int64
	Kind      string
	Title     string
	Requester string
	Reason    string
	Branch    string
	Amount    int64
	Compare   int64
	CreatedAt time.Time
	Age       string
}

func (s *Store) Approvals(status string, limit int, branch int64) ([]Approval, error) {
	rows, err := s.DB.Query(`
		SELECT a.id, a.kind, a.title, a.requester, COALESCE(a.reason,''), COALESCE(br.name,''),
		       a.amount, a.compare_amount, a.created_at
		FROM approvals a LEFT JOIN branches br ON br.id = a.branch_id
		WHERE a.status = ? AND (? = 0 OR a.branch_id = ?) ORDER BY a.created_at DESC LIMIT ?`, status, branch, branch, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		var a Approval
		if err := rows.Scan(&a.ID, &a.Kind, &a.Title, &a.Requester, &a.Reason, &a.Branch, &a.Amount, &a.Compare, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Age = humanAge(time.Since(a.CreatedAt))
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CountApprovals(status string, branch int64) (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM approvals WHERE status = ? AND (? = 0 OR branch_id = ?)`, status, branch, branch).Scan(&n)
	return n, err
}

func (s *Store) DecidedToday(status string, branch int64) (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM approvals WHERE status = ? AND DATE(decided_at) = CURDATE() AND (? = 0 OR branch_id = ?)`, status, branch, branch).Scan(&n)
	return n, err
}

func (s *Store) DefaultBranchID() (int64, error) {
	var id int64
	err := s.DB.QueryRow(`SELECT id FROM branches ORDER BY id LIMIT 1`).Scan(&id)
	return id, err
}

func (s *Store) DecideApproval(id int64, approve bool, decidedBy string, branch int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var kind string
	var status string
	var refID sql.NullInt64
	var apprBranch sql.NullInt64
	var amount int64
	if err := tx.QueryRow(`SELECT kind, status, ref_id, branch_id, amount FROM approvals WHERE id = ? FOR UPDATE`, id).Scan(&kind, &status, &refID, &apprBranch, &amount); err != nil {
		return err
	}
	if status != "menunggu" {
		return errors.New("persetujuan sudah diputuskan")
	}
	if branch != 0 && apprBranch.Valid && apprBranch.Int64 != branch {
		return errors.New("persetujuan milik cabang lain")
	}

	if refID.Valid {
		switch {
		case kind == "purchase" && approve:
			if _, err := tx.Exec(`UPDATE purchases SET status = 'disetujui' WHERE id = ?`, refID.Int64); err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE units SET status = 'siap' WHERE purchase_id = ?`, refID.Int64); err != nil {
				return err
			}
		case kind == "purchase" && !approve:
			if _, err := tx.Exec(`UPDATE purchases SET status = 'ditolak' WHERE id = ?`, refID.Int64); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM units WHERE purchase_id = ?`, refID.Int64); err != nil {
				return err
			}
		case kind == "cash" && approve:
			if _, err := tx.Exec(`UPDATE cash_sessions SET status = 'tutup', closed_at = COALESCE(closed_at, NOW()) WHERE id = ?`, refID.Int64); err != nil {
				return err
			}
		case kind == "cash" && !approve:
			if _, err := tx.Exec(`UPDATE cash_sessions SET status = 'buka', expected_balance = NULL, counted_balance = NULL, difference = NULL WHERE id = ?`, refID.Int64); err != nil {
				return err
			}
		case kind == "price" && approve:
			if _, err := tx.Exec(`UPDATE units SET min_price = ? WHERE id = ? AND status = 'siap'`, amount, refID.Int64); err != nil {
				return err
			}
		}
	}

	newStatus := "ditolak"
	if approve {
		newStatus = "disetujui"
	}
	if _, err := tx.Exec(`UPDATE approvals SET status = ?, decided_at = NOW(), decided_by = ? WHERE id = ?`, newStatus, decidedBy, id); err != nil {
		return err
	}
	return tx.Commit()
}

type PurchaseInput struct {
	BranchID    int64
	ModelID     int64
	VariantID   int64
	Prefix      string
	Title       string
	Grade       string
	SellerName  string
	SellerPhone string
	Serial      string
	Accessories []string
	DealPrice   int64
	EstCost     int64
	Method      string
	Recommended int64
	MinPrice    int64
	BuyLimit    int64
	Requester   string
	Photo       string
	Override    bool
	OverrideWhy string
}

func (s *Store) CreatePurchase(in PurchaseInput) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if in.Serial != "" && !in.Override {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM units WHERE serial = ?`, in.Serial).Scan(&n); err != nil {
			return 0, err
		}
		if n > 0 {
			return 0, ErrDuplicateSerial
		}
	}
	status := "disetujui"
	unitStatus := "siap"
	if in.DealPrice > in.BuyLimit {
		status = "menunggu"
		unitStatus = "menunggu"
	}
	acc, _ := json.Marshal(in.Accessories)
	cost := in.DealPrice + in.EstCost

	res, err := tx.Exec(`INSERT INTO purchases
		(branch_id, model_id, variant_id, grade, seller_name, seller_phone, deal_price, est_cost, status, created_at, photo, serial_override_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NULLIF(?, ''), NULLIF(?, ''))`,
		in.BranchID, in.ModelID, in.VariantID, in.Grade, in.SellerName, in.SellerPhone, in.DealPrice, in.EstCost, status, in.Photo, in.OverrideWhy)
	if err != nil {
		return 0, err
	}
	purchaseID, _ := res.LastInsertId()

	res, err = tx.Exec(`INSERT INTO units
		(code, purchase_id, branch_id, model_id, variant_id, grade, accessories, serial, cost, listed_price, min_price, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		fmt.Sprintf("%s-TMP%d", in.Prefix, time.Now().UnixNano()), purchaseID, in.BranchID, in.ModelID, in.VariantID,
		in.Grade, string(acc), in.Serial, cost, in.Recommended, in.MinPrice, unitStatus)
	if err != nil {
		return 0, err
	}
	unitID, _ := res.LastInsertId()
	if _, err := tx.Exec(`UPDATE units SET code = ? WHERE id = ?`, fmt.Sprintf("%s-%04d", in.Prefix, unitID), unitID); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(`INSERT INTO price_observations
		(model_id, variant_id, grade, series, source, price, accessories, observed_at, note)
		VALUES (?, ?, ?, 'buy', 'penjual', ?, ?, NOW(), ?)`,
		in.ModelID, in.VariantID, in.Grade, in.DealPrice, string(acc), "pembelian "+in.SellerName); err != nil {
		return 0, err
	}

	if in.Method == "" {
		in.Method = "tunai"
	}
	var sessionID sql.NullInt64
	_ = tx.QueryRow(`SELECT id FROM cash_sessions WHERE branch_id = ? AND status = 'buka' ORDER BY id DESC LIMIT 1`, in.BranchID).Scan(&sessionID)
	if _, err := tx.Exec(`INSERT INTO cash_movements (session_id, branch_id, kind, method, amount, note, created_by, created_at)
		VALUES (?, ?, 'purchase', ?, ?, ?, ?, NOW())`,
		sessionID, in.BranchID, in.Method, in.DealPrice, "pembelian "+in.SellerName, in.Requester); err != nil {
		return 0, err
	}

	if status == "menunggu" {
		reason := fmt.Sprintf("Deal %d di atas batas beli %d", in.DealPrice, in.BuyLimit)
		if _, err := tx.Exec(`INSERT INTO approvals
			(kind, ref_id, branch_id, title, requester, reason, amount, compare_amount, status, created_at)
			VALUES ('purchase', ?, ?, ?, ?, ?, ?, ?, 'menunggu', NOW())`,
			purchaseID, in.BranchID, in.Title, in.Requester, reason, in.DealPrice, in.BuyLimit); err != nil {
			return 0, err
		}
	}

	return unitID, tx.Commit()
}

type SaleInput struct {
	UnitID    int64
	Price     int64
	Discount  int64
	Method    string
	BuyerName string
	Warranty  int
	By        string
}

func (s *Store) CreateSale(in SaleInput) (string, int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()

	var cost, minPrice int64
	var status string
	var branchID, modelID, variantID int64
	var grade string
	var acc sql.NullString
	err = tx.QueryRow(`SELECT cost, min_price, status, branch_id, model_id, variant_id, grade, accessories
		FROM units WHERE id = ? FOR UPDATE`, in.UnitID).
		Scan(&cost, &minPrice, &status, &branchID, &modelID, &variantID, &grade, &acc)
	if err == sql.ErrNoRows {
		return "", 0, ErrUnitNotFound
	}
	if err != nil {
		return "", 0, err
	}
	if status != "siap" {
		return "", 0, ErrUnitNotReady
	}
	if in.Price < minPrice {
		return "", 0, ErrBelowMin
	}

	profit := in.Price - in.Discount - cost
	res, err := tx.Exec(`INSERT INTO sales
		(invoice, unit_id, branch_id, price, discount, method, buyer_name, sold_by, warranty_days, profit, sold_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		fmt.Sprintf("TMP-%d", time.Now().UnixNano()), in.UnitID, branchID, in.Price, in.Discount, in.Method,
		in.BuyerName, in.By, in.Warranty, profit)
	if err != nil {
		return "", 0, err
	}
	saleID, _ := res.LastInsertId()
	invoice := fmt.Sprintf("INV-%05d", saleID)
	if _, err := tx.Exec(`UPDATE sales SET invoice = ? WHERE id = ?`, invoice, saleID); err != nil {
		return "", 0, err
	}
	if _, err := tx.Exec(`UPDATE units SET status = 'terjual', sold_at = NOW() WHERE id = ?`, in.UnitID); err != nil {
		return "", 0, err
	}
	var sessionID sql.NullInt64
	_ = tx.QueryRow(`SELECT id FROM cash_sessions WHERE branch_id = ? AND status = 'buka' ORDER BY id DESC LIMIT 1`, branchID).Scan(&sessionID)
	if _, err := tx.Exec(`INSERT INTO cash_movements (session_id, branch_id, kind, method, amount, note, created_by, created_at)
		VALUES (?, ?, 'sale', ?, ?, ?, ?, NOW())`,
		sessionID, branchID, in.Method, in.Price-in.Discount, invoice, in.By); err != nil {
		return "", 0, err
	}
	if _, err := tx.Exec(`INSERT INTO price_observations
		(model_id, variant_id, grade, series, source, price, accessories, observed_at, note)
		VALUES (?, ?, ?, 'sell', 'toko', ?, ?, NOW(), ?)`,
		modelID, variantID, grade, in.Price-in.Discount, acc.String, "penjualan "+invoice); err != nil {
		return "", 0, err
	}
	return invoice, profit, tx.Commit()
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d menit lalu", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d jam lalu", int(d.Hours()))
	default:
		return fmt.Sprintf("%d hari lalu", int(d.Hours()/24))
	}
}

func bp(v string) float64 {
	n, _ := strconv.ParseFloat(v, 64)
	return n / 10000
}

func atoi(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
