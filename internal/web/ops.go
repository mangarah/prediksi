package web

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"prediksi/internal/pricing"
	"prediksi/internal/store"
)

// ---------- Tukar tambah ----------

type tradeInData struct {
	Units         []store.Unit
	Out           *store.Unit
	OutPrice      int64
	SalePrice     int64
	Diff          int64
	In            *quoteView
	Method        string
	NeedsApproval bool
}

func (s *Server) TukarTambah(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "kasir"); !ok {
		return
	}
	branch := s.branchScope(r)
	units, err := s.st.ReadyUnits(branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	qv, err := s.buildQuote(r.URL.Query())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := &tradeInData{Units: units, In: qv, Method: "tunai", SalePrice: parseMoney(r.URL.Query().Get("sale_price"))}
	if len(units) > 0 {
		d.Out = &units[0]
		if id := parseMoney(r.URL.Query().Get("unit")); id > 0 {
			for i := range units {
				if units[i].ID == id {
					d.Out = &units[i]
				}
			}
		}
		d.OutPrice = d.Out.ListedPrice
		if d.SalePrice == 0 {
			d.SalePrice = d.Out.ListedPrice
		}
	}
	if qv.Deal > 0 && d.Out != nil {
		d.Diff = d.SalePrice - qv.Deal
		d.NeedsApproval = qv.Q.LowData || qv.Deal > qv.Q.BuyLimit
	}
	v := s.view(r, "Tukar tambah", "tukar")
	v.Data = d
	s.render(w, "tukartambah", v)
}

func (s *Server) SimpanTukarTambah(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer", "kasir")
	if !ok {
		return
	}
	_ = r.ParseForm()
	out, err := s.st.UnitByID(parseMoney(r.FormValue("unit_id")))
	if err != nil {
		http.Redirect(w, r, "/tukar-tambah?err="+url.QueryEscape("Unit keluar tidak ditemukan."), http.StatusFound)
		return
	}
	if b := branchOf(u); b != 0 && out.BranchID != b {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}
	qv, err := s.buildQuote(r.Form)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	value := qv.Deal
	if value <= 0 {
		http.Redirect(w, r, "/tukar-tambah?err="+url.QueryEscape("Nilai tukar wajib diisi."), http.StatusFound)
		return
	}
	salePrice := parseMoney(r.FormValue("sale_price"))
	if salePrice == 0 {
		salePrice = out.ListedPrice
	}
	discount := parseMoney(r.FormValue("discount"))
	if salePrice < out.MinPrice {
		msg := fmt.Sprintf("Harga jual unit keluar di bawah minimal %s.", rp(out.MinPrice))
		http.Redirect(w, r, "/tukar-tambah?err="+url.QueryEscape(msg), http.StatusFound)
		return
	}
	serial := strings.TrimSpace(r.FormValue("serial"))
	if exists, _ := s.st.SerialExists(serial); exists {
		http.Redirect(w, r, "/tukar-tambah?err="+url.QueryEscape("Nomor seri "+serial+" sudah ada di sistem."), http.StatusFound)
		return
	}
	method := r.FormValue("method")
	if method != "tunai" && method != "transfer" && method != "qris" {
		method = "tunai"
	}
	buyer := strings.TrimSpace(r.FormValue("buyer"))
	if buyer == "" {
		buyer = "Pelanggan tukar tambah"
	}
	var accs []string
	for _, a := range qv.Accessories {
		if qv.Checked[a.Code] {
			accs = append(accs, a.Code)
		}
	}
	saleID, _, diff, pending, err := s.st.CreateTradeIn(store.TradeInInput{
		BranchID: out.BranchID, OutUnitID: out.ID,
		SalePrice: salePrice, Discount: discount, Method: method, Buyer: buyer, By: u.Name,
		InModelID: qv.Selected.ID, InVariantID: qv.Selected.VariantID, InPrefix: qv.Selected.Prefix,
		InTitle: fmt.Sprintf("%s %s · Grade %s (tukar tambah)", qv.Selected.Name, qv.Selected.Variant, qv.Grade),
		InGrade: qv.Grade, InSerial: serial, InAccessories: accs, InValue: value, EstCost: qv.EstCost,
		Recommended: qv.Q.Recommended, MinPrice: qv.Q.MinPrice, BuyLimit: qv.Q.BuyLimit,
		Requester: u.Name,
	})
	if err != nil {
		msg := "Gagal menyimpan tukar tambah."
		if err == store.ErrBelowMin {
			msg = "Harga jual di bawah harga minimal."
		}
		http.Redirect(w, r, "/tukar-tambah?err="+url.QueryEscape(msg), http.StatusFound)
		return
	}
	s.st.Audit(u.Name, "tukar tambah", "sale", saleID, "", fmt.Sprintf("nilai %d selisih %d", value, diff))
	msg := fmt.Sprintf("Tukar tambah tersimpan. Selisih %s.", signedDiff(salePrice-discount, value))
	if pending {
		msg += " Unit masuk menunggu persetujuan."
		s.notify(fmt.Sprintf("Persetujuan tukar tambah: %s %s nilai %s (batas %s). Oleh %s.",
			qv.Selected.Name, qv.Selected.Variant, rp(value), rp(qv.Q.BuyLimit), u.Name))
	}
	http.Redirect(w, r, fmt.Sprintf("/kasir?ok=%s", url.QueryEscape(msg)), http.StatusFound)
}

// ---------- Transfer ----------

type transferData struct {
	Units     []store.Unit
	Branches  []store.Branch
	Transfers []store.Transfer
}

func (s *Server) TransferPage(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	branch := branchOf(u)
	units, err := s.st.ReadyUnits(branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	branches, err := s.st.Branches()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ts, err := s.st.Transfers(branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(r, "Transfer", "transfer")
	v.Data = &transferData{Units: units, Branches: branches, Transfers: ts}
	s.render(w, "transfer", v)
}

func (s *Server) KirimTransfer(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	_ = r.ParseForm()
	unitID := parseMoney(r.FormValue("unit_id"))
	to := parseMoney(r.FormValue("to_branch"))
	unit, err := s.st.UnitByID(unitID)
	if err != nil {
		http.Redirect(w, r, "/transfer?err="+url.QueryEscape("Unit tidak ditemukan."), http.StatusFound)
		return
	}
	if b := branchOf(u); b != 0 && unit.BranchID != b {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}
	if to == 0 || to == unit.BranchID {
		http.Redirect(w, r, "/transfer?err="+url.QueryEscape("Pilih cabang tujuan lain."), http.StatusFound)
		return
	}
	if err := s.st.CreateTransfer(unitID, to, u.Name); err != nil {
		http.Redirect(w, r, "/transfer?err="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	s.st.Audit(u.Name, "kirim", "transfer", unitID, "", fmt.Sprintf("%d→%d", unit.BranchID, to))
	http.Redirect(w, r, "/transfer?ok="+url.QueryEscape("Unit "+unit.Code+" dikirim."), http.StatusFound)
}

func (s *Server) TerimaTransfer(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	id := parseMoney(chi.URLParam(r, "id"))
	t, err := s.st.TransferByID(id)
	if err != nil {
		http.Redirect(w, r, "/transfer?err="+url.QueryEscape("Transfer tidak ditemukan."), http.StatusFound)
		return
	}
	if b := branchOf(u); b != 0 && t.ToID != b {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}
	if err := s.st.ReceiveTransfer(id, u.Name); err != nil {
		http.Redirect(w, r, "/transfer?err="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	s.st.Audit(u.Name, "terima", "transfer", id, "", t.UnitCode)
	http.Redirect(w, r, "/transfer?ok="+url.QueryEscape("Unit "+t.UnitCode+" diterima."), http.StatusFound)
}

// ---------- Laporan ----------

type laporanData struct {
	Rep   store.Report
	From  string
	To    string
	Scope int64
}

func dateOrDefault(s string, def time.Time) time.Time {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return def
}

func (s *Server) Laporan(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	now := time.Now()
	from := dateOrDefault(r.URL.Query().Get("from"), time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local))
	to := dateOrDefault(r.URL.Query().Get("to"), now)
	rep, err := s.st.Report(from, to, branchOf(u))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(r, "Laporan", "laporan")
	v.Data = &laporanData{Rep: rep, From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), Scope: branchOf(u)}
	s.render(w, "laporan", v)
}

func (s *Server) LaporanCSV(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	now := time.Now()
	from := dateOrDefault(r.URL.Query().Get("from"), time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local))
	to := dateOrDefault(r.URL.Query().Get("to"), now)
	lines, err := s.st.SaleLines(from, to, branchOf(u))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="laporan-%s.csv"`, time.Now().Format("20060102")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"invoice", "tanggal", "cabang", "model", "varian", "grade", "serial", "pembeli", "kasir", "metode", "harga", "diskon", "total", "modal", "laba"})
	for _, l := range lines {
		_ = cw.Write([]string{l.Invoice, l.Date, l.Branch, l.Model, l.Variant, l.Grade, l.Serial, l.Buyer, l.SoldBy, l.Method,
			strconv.FormatInt(l.Price, 10), strconv.FormatInt(l.Discount, 10), strconv.FormatInt(l.Total, 10),
			strconv.FormatInt(l.Cost, 10), strconv.FormatInt(l.Profit, 10)})
	}
	cw.Flush()
}

// ---------- Prediksi ----------

type demandData struct {
	Rows []demandRowView
	Top  []demandRowView
	Days int
}

type demandRowView struct {
	store.DemandRow
	BuyLimit    int64
	Recommended int64
	HasQuote    bool
	Forecast4   int
	Low4        int
	High4       int
}

func (s *Server) Demand(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	rows, err := s.st.DemandRows(30, branchOf(u))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	set, err := s.st.Settings()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := &demandData{Days: 30}
	for i, row := range rows {
		rv := demandRowView{DemandRow: row}
		rv.Forecast4 = int(math.Round(row.Weekly * 4))
		rv.Low4 = int(math.Round(float64(rv.Forecast4) * 0.7))
		rv.High4 = int(math.Round(float64(rv.Forecast4) * 1.3))
		if i < 15 {
			target := set.TargetMargin[row.Category]
			if target <= 0 {
				target = 0.12
			}
			obs, err := s.st.Observations(row.ModelID, row.VariantID, set.PeriodDays)
			if err == nil {
				q := pricing.Quote(obs, "A", pricing.Params{
					TargetMargin: target, MinMargin: set.MinMargin, EstCost: set.EstCost,
					GradeFactor: set.GradeFactors["A"], Weights: set.Weights,
					HalfLifeDays: set.HalfLifeDays, Now: time.Now(),
				})
				if q.Count > 0 {
					rv.HasQuote = true
					rv.BuyLimit = q.BuyLimit
					rv.Recommended = q.Recommended
				}
			}
		}
		d.Rows = append(d.Rows, rv)
		if row.Score >= 70 && row.Need > 0 && rv.HasQuote {
			d.Top = append(d.Top, rv)
		}
	}
	v := s.view(r, "Prediksi & saran beli", "demand")
	v.Data = d
	s.render(w, "demand", v)
}

// ---------- Label ----------

func (s *Server) Label(w http.ResponseWriter, r *http.Request) {
	u := userOf(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	unit, err := s.st.UnitByID(parseMoney(chi.URLParam(r, "id")))
	if err != nil {
		http.Error(w, "Unit tidak ditemukan", http.StatusNotFound)
		return
	}
	if b := branchOf(u); b != 0 && unit.BranchID != b {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages["label"].ExecuteTemplate(w, "label.html", map[string]any{"Unit": unit}); err != nil {
		s.fail(w, r, err)
	}
}

// ---------- Opname ----------

type opnameData struct {
	Units    []store.Unit
	Branches []store.Branch
	BranchID int64
	Codes    string
	Result   *store.OpnameResult
}

func (s *Server) Opname(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	branch := s.kasBranch(r)
	if branch == 0 {
		branch, _ = s.st.DefaultBranchID()
	}
	branches, _ := s.st.Branches()
	d := &opnameData{Branches: branches, BranchID: branch}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		d.Codes = r.FormValue("codes")
		codes := strings.FieldsFunc(d.Codes, func(c rune) bool { return c == '\n' || c == '\r' || c == ',' || c == ';' })
		res, err := s.st.Opname(branch, codes)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		d.Result = &res
		s.st.Audit(u.Name, "opname", "stok", branch, "", fmt.Sprintf("hilang %d, lebih %d", len(res.Missing), len(res.Extra)))
	}
	units, err := s.st.ReadyUnits(branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d.Units = units
	v := s.view(r, "Stok opname", "opname")
	v.Data = d
	s.render(w, "opname", v)
}

// ---------- Impor ----------

type imporRow struct {
	Line  int
	Cells []string
	OK    bool
	Msg   string
}

type imporData struct {
	Kind     string
	CSV      string
	Preview  []imporRow
	Errors   []string
	Inserted int
	Done     bool
}

func (s *Server) Impor(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	d := &imporData{Kind: "harga"}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		d.Kind = r.FormValue("kind")
		if d.Kind != "stok" {
			d.Kind = "harga"
		}
		d.CSV = r.FormValue("csv")
		rows, errs, parsedObs, parsedUnits := s.parseImport(d.Kind, d.CSV)
		d.Preview, d.Errors = rows, errs
		if r.FormValue("confirm") == "1" && len(errs) == 0 {
			if d.Kind == "harga" {
				for _, o := range parsedObs {
					if _, err := s.st.AddObservation(o); err != nil {
						d.Errors = append(d.Errors, err.Error())
						continue
					}
					d.Inserted++
				}
			} else {
				n, errs, err := s.st.ImportUnits(parsedUnits)
				if err != nil {
					d.Errors = append(d.Errors, err.Error())
				}
				d.Inserted = n
				d.Errors = append(d.Errors, errs...)
			}
			d.Done = true
			s.st.Audit(u.Name, "impor", d.Kind, 0, "", fmt.Sprintf("%d baris", d.Inserted))
		}
	}
	v := s.view(r, "Impor data", "impor")
	v.Data = d
	s.render(w, "impor", v)
}

func savePhoto(r *http.Request) (string, error) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return "", nil
	}
	file, hdr, err := r.FormFile("photo")
	if err == http.ErrMissingFile {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("foto tidak terbaca")
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return "", fmt.Errorf("foto harus jpg/png/webp")
	}
	dir := filepath.Join("data", "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}

func (s *Server) parseImport(kind, text string) ([]imporRow, []string, []store.NewObservation, []store.ImportUnit) {
	var preview []imporRow
	var errs []string
	var obs []store.NewObservation
	var units []store.ImportUnit
	models, err := s.st.Models()
	if err != nil {
		return nil, []string{err.Error()}, nil, nil
	}
	pairs := map[string]store.Model{}
	for _, m := range models {
		pairs[strings.ToLower(m.Name+"|"+m.Variant)] = m
	}
	rd := csv.NewReader(strings.NewReader(strings.TrimSpace(text)))
	rd.FieldsPerRecord = -1
	records, err := rd.ReadAll()
	if err != nil {
		return nil, []string{"CSV tidak valid: " + err.Error()}, nil, nil
	}
	if len(records) < 2 {
		return nil, []string{"CSV minimal berisi header dan 1 baris data."}, nil, nil
	}
	start := 0
	if strings.EqualFold(records[0][0], "model") {
		start = 1
	}
	for i := start; i < len(records); i++ {
		rec := records[i]
		pr := imporRow{Line: i + 1, Cells: rec, OK: true}
		get := func(idx int) string {
			if idx < len(rec) {
				return strings.TrimSpace(rec[idx])
			}
			return ""
		}
		if kind == "harga" {
			if len(rec) < 6 {
				pr.OK, pr.Msg = false, "butuh 6 kolom: model,varian,grade,series,source,price"
			} else {
				m, ok := pairs[strings.ToLower(get(0)+"|"+get(1))]
				grade, series, source := get(2), get(3), get(4)
				price := parseMoney(get(5))
				switch {
				case !ok:
					pr.OK, pr.Msg = false, "model/varian tidak dikenal"
				case !strings.Contains("ABCD", grade) || len(grade) != 1:
					pr.OK, pr.Msg = false, "grade harus A-D"
				case series != "sell" && series != "buy":
					pr.OK, pr.Msg = false, "series harus sell/buy"
				case !validSource(source):
					pr.OK, pr.Msg = false, "source tidak valid"
				case price <= 0:
					pr.OK, pr.Msg = false, "harga wajib > 0"
				default:
					obs = append(obs, store.NewObservation{
						ModelID: m.ID, VariantID: m.VariantID, Grade: grade, Series: series, Source: source,
						Price: price, Accessories: nil, ObservedAt: time.Now(), Note: get(6) + " (impor)",
					})
				}
			}
		} else {
			if len(rec) < 8 {
				pr.OK, pr.Msg = false, "butuh 8 kolom: model,varian,grade,serial,cost,listed,min,branch"
			} else {
				m, ok := pairs[strings.ToLower(get(0)+"|"+get(1))]
				grade := get(2)
				cost, listed, min := parseMoney(get(4)), parseMoney(get(5)), parseMoney(get(6))
				switch {
				case !ok:
					pr.OK, pr.Msg = false, "model/varian tidak dikenal"
				case !strings.Contains("ABCD", grade) || len(grade) != 1:
					pr.OK, pr.Msg = false, "grade harus A-D"
				case cost <= 0 || listed <= 0:
					pr.OK, pr.Msg = false, "modal dan harga jual wajib > 0"
				case get(7) == "":
					pr.OK, pr.Msg = false, "cabang wajib diisi"
				default:
					units = append(units, store.ImportUnit{
						Model: m.Name, Variant: m.Variant, Grade: grade, Serial: get(3),
						Cost: cost, Listed: listed, Min: min, Branch: get(7),
					})
				}
			}
		}
		if !pr.OK {
			errs = append(errs, fmt.Sprintf("baris %d: %s", pr.Line, pr.Msg))
		}
		preview = append(preview, pr)
	}
	return preview, errs, obs, units
}
