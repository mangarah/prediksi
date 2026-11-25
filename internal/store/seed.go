package store

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type seedObs struct {
	model, variant, grade, series, source string
	price                                 int64
	daysAgo                               int
}

type seedUnit struct {
	model, variant, grade, serial string
	cost, listed, min             int64
	status                        string
	daysAgo                       int
	soldDaysAgo                   int // >=0 berarti unit terjual
	soldPrice                     int64
	method                        string
}

type seedApproval struct {
	kind, title, requester, reason string
	amount, compare                int64
	minutesAgo                     int
}

// Seed mengisi data contoh sekali saja, saat tabel branches masih kosong.
func (s *Store) Seed() error {
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM branches`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	exec := func(q string, args ...any) (int64, error) {
		r, err := tx.Exec(q, args...)
		if err != nil {
			return 0, err
		}
		return r.LastInsertId()
	}

	pusat, err := exec(`INSERT INTO branches (name) VALUES ('Cabang Pusat')`)
	if err != nil {
		return err
	}
	cabangA, err := exec(`INSERT INTO branches (name) VALUES ('Cabang A')`)
	if err != nil {
		return err
	}

	users := []struct {
		name, email, pass, role string
		branch                  any
	}{
		{"Kazoku", "owner@toko.local", "owner123", "owner", nil},
		{"Rudi", "manajer@toko.local", "manajer123", "manajer", cabangA},
		{"Sari", "kasir@toko.local", "kasir123", "kasir", pusat},
		{"Bayu", "buyer@toko.local", "buyer123", "buyer", pusat},
	}
	for _, u := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.pass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := exec(`INSERT INTO users (name, email, password_hash, role, branch_id, active, created_at)
			VALUES (?, ?, ?, ?, ?, 1, NOW())`, u.name, u.email, string(hash), u.role, u.branch); err != nil {
			return err
		}
	}

	catKonsol, err := exec(`INSERT INTO categories (name, target_margin_bp, sort_order) VALUES ('Konsol', 1200, 1)`)
	if err != nil {
		return err
	}
	catGame, err := exec(`INSERT INTO categories (name, target_margin_bp, sort_order) VALUES ('Game', 2500, 2)`)
	if err != nil {
		return err
	}
	catAcc, err := exec(`INSERT INTO categories (name, target_margin_bp, sort_order) VALUES ('Aksesoris', 3000, 3)`)
	if err != nil {
		return err
	}

	brands := map[string]int64{}
	for _, b := range []struct {
		name string
		cat  int64
	}{{"Sony", catKonsol}, {"Nintendo", catKonsol}, {"Sony Game", catGame}, {"Sony Acc", catAcc}, {"8BitDo", catAcc}} {
		id, err := exec(`INSERT INTO brands (category_id, name) VALUES (?, ?)`, b.cat, b.name)
		if err != nil {
			return err
		}
		brands[b.name] = id
	}

	models := map[string]int64{}
	modelVariants := map[string]int64{}
	modelPrefix := map[string]string{}
	for _, m := range []struct {
		brand, name, prefix, variant string
	}{{"Sony", "PS5 Slim", "PS5", "Disc 1TB"}, {"Sony", "PS5 Slim", "PS5", "Digital 825GB"},
		{"Sony", "PS4 Pro", "PS4", "1TB"}, {"Nintendo", "Switch OLED", "NSW", "64GB"},
		{"Nintendo", "Switch Lite", "NSW", "32GB"}, {"Sony Game", "Spider-Man 2", "GME", "Kaset"},
		{"Sony Game", "God of War Ragnarok", "GME", "Kaset"}, {"Sony Acc", "DualSense Controller", "ACC", "Putih"},
		{"8BitDo", "Pro 2 Controller", "ACC", "Hitam"}} {
		key := m.name + "|" + m.variant
		if _, ok := models[m.name]; !ok {
			id, err := exec(`INSERT INTO product_models (brand_id, name, prefix) VALUES (?, ?, ?)`, brands[m.brand], m.name, m.prefix)
			if err != nil {
				return err
			}
			models[m.name] = id
			modelPrefix[m.name] = m.prefix
		}
		id, err := exec(`INSERT INTO variants (model_id, name) VALUES (?, ?)`, models[m.name], m.variant)
		if err != nil {
			return err
		}
		modelVariants[key] = id
	}

	if _, err := tx.Exec(`INSERT INTO grades (code, label, factor_bp) VALUES
		('A','Mulus',10000),('B','Lecet ringan',9000),('C','Ada minus',7800),('D','Rusak / part',5000)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO accessories (code, label, deduction) VALUES
		('dus','Dus',150000),('stik','Stik',400000),('kabel','Kabel',50000),
		('charger','Charger',75000),('nota','Nota',25000),('garansi','Garansi',50000)`); err != nil {
		return err
	}

	settings := map[string]string{
		"min_margin_bp": "500", "est_cost": "100000", "period_days": "60", "half_life_days": "30",
		"cash_threshold": "10000",
		"w_toko":         "10000", "w_pesaing": "8000", "w_marketplace": "6000", "w_penjual": "10000",
	}
	for k, v := range settings {
		if _, err := exec(`INSERT INTO settings (k, v) VALUES (?, ?)`, k, v); err != nil {
			return err
		}
	}

	obs := []seedObs{
		{"PS5 Slim", "Disc 1TB", "A", "sell", "pesaing", 7_000_000, 40},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "marketplace", 7_100_000, 32},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "toko", 7_150_000, 26},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "toko", 7_200_000, 20},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "marketplace", 7_200_000, 18},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "pesaing", 7_250_000, 15},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "toko", 7_300_000, 12},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "marketplace", 7_350_000, 10},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "pesaing", 7_400_000, 8},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "toko", 7_450_000, 6},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "toko", 7_200_000, 4},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "pesaing", 7_250_000, 3},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "marketplace", 7_200_000, 2},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "toko", 7_200_000, 1},
		{"PS5 Slim", "Disc 1TB", "A", "sell", "marketplace", 12_900_000, 5},
		{"PS5 Slim", "Disc 1TB", "B", "sell", "toko", 6_500_000, 22},
		{"PS5 Slim", "Disc 1TB", "B", "sell", "pesaing", 6_600_000, 17},
		{"PS5 Slim", "Digital 825GB", "A", "sell", "toko", 6_900_000, 24},
		{"PS5 Slim", "Digital 825GB", "A", "sell", "marketplace", 7_000_000, 19},
		{"PS5 Slim", "Digital 825GB", "A", "sell", "pesaing", 7_100_000, 12},
		{"PS5 Slim", "Digital 825GB", "A", "sell", "toko", 7_000_000, 7},
		{"PS5 Slim", "Digital 825GB", "A", "sell", "toko", 7_050_000, 3},
		{"PS4 Pro", "1TB", "A", "sell", "toko", 2_700_000, 31},
		{"PS4 Pro", "1TB", "A", "sell", "pesaing", 2_800_000, 24},
		{"PS4 Pro", "1TB", "A", "sell", "marketplace", 2_850_000, 18},
		{"PS4 Pro", "1TB", "A", "sell", "toko", 2_900_000, 11},
		{"PS4 Pro", "1TB", "A", "sell", "toko", 2_950_000, 5},
		{"PS4 Pro", "1TB", "A", "sell", "pesaing", 3_000_000, 2},
		{"Switch OLED", "64GB", "A", "sell", "toko", 4_600_000, 35},
		{"Switch OLED", "64GB", "A", "sell", "marketplace", 4_700_000, 28},
		{"Switch OLED", "64GB", "A", "sell", "pesaing", 4_750_000, 21},
		{"Switch OLED", "64GB", "A", "sell", "toko", 4_800_000, 14},
		{"Switch OLED", "64GB", "A", "sell", "toko", 4_850_000, 9},
		{"Switch OLED", "64GB", "A", "sell", "pesaing", 4_900_000, 4},
		{"Switch OLED", "64GB", "A", "sell", "toko", 4_700_000, 1},
		{"Spider-Man 2", "Kaset", "A", "sell", "toko", 600_000, 30},
		{"Spider-Man 2", "Kaset", "A", "sell", "pesaing", 620_000, 22},
		{"Spider-Man 2", "Kaset", "A", "sell", "marketplace", 640_000, 16},
		{"Spider-Man 2", "Kaset", "A", "sell", "toko", 650_000, 9},
		{"Spider-Man 2", "Kaset", "A", "sell", "toko", 660_000, 3},
		{"DualSense Controller", "Putih", "B", "sell", "toko", 700_000, 26},
		{"DualSense Controller", "Putih", "B", "sell", "marketplace", 720_000, 18},
		{"DualSense Controller", "Putih", "B", "sell", "pesaing", 740_000, 12},
		{"DualSense Controller", "Putih", "B", "sell", "toko", 750_000, 6},
		{"DualSense Controller", "Putih", "B", "sell", "toko", 760_000, 2},
		{"Switch Lite", "32GB", "B", "sell", "toko", 2_250_000, 20},
		{"Switch Lite", "32GB", "B", "sell", "pesaing", 2_300_000, 15},
		{"Switch Lite", "32GB", "B", "sell", "marketplace", 2_350_000, 10},
		{"Switch Lite", "32GB", "B", "sell", "toko", 2_400_000, 5},
		{"Switch Lite", "32GB", "B", "sell", "toko", 2_350_000, 1},
		{"Pro 2 Controller", "Hitam", "B", "sell", "toko", 780_000, 25},
		{"Pro 2 Controller", "Hitam", "B", "sell", "pesaing", 800_000, 18},
		{"Pro 2 Controller", "Hitam", "B", "sell", "marketplace", 820_000, 12},
		{"Pro 2 Controller", "Hitam", "B", "sell", "toko", 850_000, 6},
		{"Pro 2 Controller", "Hitam", "B", "sell", "toko", 790_000, 3},
		{"Pro 2 Controller", "Hitam", "B", "sell", "pesaing", 810_000, 1},
	}
	accJSON := `["dus","stik","kabel","charger"]`
	for _, o := range obs {
		vid, ok := modelVariants[o.model+"|"+o.variant]
		if !ok {
			return fmt.Errorf("seed: variant %s|%s tidak ada", o.model, o.variant)
		}
		if _, err := exec(`INSERT INTO price_observations
			(model_id, variant_id, grade, series, source, price, accessories, observed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			models[o.model], vid, o.grade, o.series, o.source, o.price, accJSON,
			time.Now().AddDate(0, 0, -o.daysAgo)); err != nil {
			return err
		}
	}

	units := []seedUnit{
		{"PS5 Slim", "Disc 1TB", "A", "SN 8842-1190", 5_900_000, 7_200_000, 6_200_000, "siap", 4, -1, 0, ""},
		{"Switch OLED", "64GB", "B", "SN 7712-0043", 3_450_000, 4_700_000, 3_650_000, "siap", 12, -1, 0, ""},
		{"PS4 Pro", "1TB", "C", "SN 5521-8890", 2_100_000, 2_850_000, 2_250_000, "servis", 9, -1, 0, ""},
		{"Spider-Man 2", "Kaset", "A", "Kaset lengkap", 420_000, 650_000, 450_000, "siap", 31, -1, 0, ""},
		{"PS5 Slim", "Digital 825GB", "A", "SN 8843-0021", 6_350_000, 7_550_000, 6_700_000, "menunggu", 1, -1, 0, ""},
		{"DualSense Controller", "Putih", "B", "Tanpa dus", 520_000, 780_000, 560_000, "siap", 22, -1, 0, ""},
		{"Pro 2 Controller", "Hitam", "B", "Lengkap", 610_000, 850_000, 650_000, "terjual", 5, 0, 850_000, "tunai"},
		{"Switch OLED", "64GB", "A", "SN 7705-0032", 3_900_000, 5_100_000, 4_100_000, "terjual", 9, 1, 5_100_000, "transfer"},
		{"Spider-Man 2", "Kaset", "A", "Kaset", 380_000, 650_000, 410_000, "terjual", 8, 1, 650_000, "qris"},
		{"PS5 Slim", "Disc 1TB", "A", "SN 8810-0088", 5_750_000, 7_250_000, 6_050_000, "terjual", 12, 2, 7_400_000, "tunai"},
		{"PS4 Pro", "1TB", "B", "SN 5502-0121", 1_950_000, 2_700_000, 2_050_000, "terjual", 20, 3, 2_800_000, "tunai"},
		{"DualSense Controller", "Putih", "B", "Lengkap", 480_000, 780_000, 510_000, "terjual", 15, 4, 800_000, "qris"},
		{"Switch Lite", "32GB", "B", "SN 4410-0077", 1_750_000, 2_400_000, 1_850_000, "terjual", 18, 4, 2_450_000, "tunai"},
		{"God of War Ragnarok", "Kaset", "A", "Kaset", 350_000, 600_000, 380_000, "terjual", 22, 5, 600_000, "tunai"},
		{"PS5 Slim", "Digital 825GB", "A", "SN 8830-0071", 6_100_000, 7_100_000, 6_450_000, "terjual", 26, 6, 7_100_000, "transfer"},
	}

	purchaseCount := 0
	type cashSeed struct {
		branch int64
		method string
		amount int64
		at     time.Time
	}
	var cashRows []cashSeed
	for _, u := range units {
		key := u.model + "|" + u.variant
		vid := modelVariants[key]
		branch := pusat
		if purchaseCount%3 == 1 {
			branch = cabangA
		}
		created := time.Now().AddDate(0, 0, -u.daysAgo).Add(-time.Hour * 3)
		status := "disetujui"
		if u.status == "menunggu" {
			status = "menunggu"
		}
		purchaseID, err := exec(`INSERT INTO purchases
			(branch_id, model_id, variant_id, grade, seller_name, seller_phone, deal_price, est_cost, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			branch, models[u.model], vid, u.grade, "Penjual Demo", "0812-0000-00", u.cost-100_000, 100_000, status, created)
		if err != nil {
			return err
		}
		purchaseCount++

		prefix := modelPrefix[u.model]
		unitID, err := exec(`INSERT INTO units
			(code, purchase_id, branch_id, model_id, variant_id, grade, accessories, serial, cost, listed_price, min_price, status, created_at, sold_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			fmt.Sprintf("%s-SEED%03d", prefix, purchaseCount), purchaseID, branch, models[u.model], vid, u.grade,
			accJSON, u.serial, u.cost, u.listed, u.min, u.status, created)
		if err != nil {
			return err
		}
		if _, err := exec(`UPDATE units SET code = ? WHERE id = ?`, fmt.Sprintf("%s-%04d", prefix, unitID), unitID); err != nil {
			return err
		}

		if u.soldDaysAgo >= 0 {
			soldAt := time.Now().AddDate(0, 0, -u.soldDaysAgo).Add(-time.Hour * time.Duration(2+unitID%5))
			profit := u.soldPrice - u.cost
			if _, err := exec(`INSERT INTO sales
				(invoice, unit_id, branch_id, price, discount, method, buyer_name, sold_by, warranty_days, profit, sold_at)
				VALUES (?, ?, ?, ?, 0, ?, '', 'Sari', 14, ?, ?)`,
				fmt.Sprintf("ARSIP-%03d", unitID), unitID, branch, u.soldPrice, u.method, profit, soldAt); err != nil {
				return err
			}
			if _, err := exec(`UPDATE units SET sold_at = ? WHERE id = ?`, soldAt, unitID); err != nil {
				return err
			}
			cashRows = append(cashRows, cashSeed{branch: branch, method: u.method, amount: u.soldPrice, at: soldAt})
		}
	}

	openAt := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 8, 0, 0, 0, time.Local)
	sessions := map[int64]int64{}
	for _, cs := range []struct {
		branch  int64
		opening int64
		by      string
	}{{pusat, 2_000_000, "Sari"}, {cabangA, 1_500_000, "Rudi"}} {
		sid, err := exec(`INSERT INTO cash_sessions (branch_id, opened_by, opening_balance, opened_at, status)
			VALUES (?, ?, ?, ?, 'buka')`, cs.branch, cs.by, cs.opening, openAt)
		if err != nil {
			return err
		}
		sessions[cs.branch] = sid
	}
	for _, m := range cashRows {
		if _, err := exec(`INSERT INTO cash_movements (session_id, branch_id, kind, method, amount, note, created_by, created_at)
			VALUES (?, ?, 'sale', ?, ?, 'penjualan contoh', 'Sari', ?)`,
			sessions[m.branch], m.branch, m.method, m.amount, m.at); err != nil {
			return err
		}
	}

	// Satu persetujuan pembelian yang menunggu, tertaut ke unit PS5 Digital.
	var ps5Digital int64
	if err := tx.QueryRow(`SELECT u.purchase_id FROM units u WHERE u.serial = 'SN 8843-0021'`).Scan(&ps5Digital); err != nil {
		return err
	}
	if _, err := exec(`INSERT INTO approvals
		(kind, ref_id, branch_id, title, requester, reason, amount, compare_amount, status, created_at)
		VALUES ('purchase', ?, ?, 'PS5 Slim Digital 825GB · Grade A', 'Bayu (Buyer)',
		        'Deal di atas batas beli maksimal', 6350000, 6020000, 'menunggu', ?)`,
		ps5Digital, pusat, time.Now().Add(-14*time.Minute)); err != nil {
		return err
	}

	others := []seedApproval{
		{"price", "DualSense Controller · Grade B", "Sari (Kasir)", "Jual di bawah harga minimal", 650_000, 676_000, 42},
		{"cash", "Tutup kas Cabang A · kemarin", "Rudi (Manajer)", "Selisih kas melebihi Rp 10.000", 4_242_000, 4_200_000, 120},
	}
	for _, a := range others {
		branch := cabangA
		if _, err := exec(`INSERT INTO approvals
			(kind, ref_id, branch_id, title, requester, reason, amount, compare_amount, status, created_at)
			VALUES (?, NULL, ?, ?, ?, ?, ?, ?, 'menunggu', ?)`,
			a.kind, branch, a.title, a.requester, a.reason, a.amount, a.compare, time.Now().Add(-time.Duration(a.minutesAgo)*time.Minute)); err != nil {
			return err
		}
	}

	return tx.Commit()
}
