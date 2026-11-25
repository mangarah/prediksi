package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"prediksi/internal/store"
)

// ---------- Data harga ----------

type dataHargaData struct {
	Models      []store.Model
	Grades      []string
	Accessories []store.Accessory
	Selected    store.Model
	Rows        []store.ObsRow
}

func (s *Server) DataHarga(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "buyer"); !ok {
		return
	}
	models, err := s.st.Models()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	accs, err := s.st.Accessories()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := &dataHargaData{Models: models, Grades: gradeCodes, Accessories: accs}
	if len(models) > 0 {
		d.Selected = models[0]
	}
	var modelID int64
	if raw := r.URL.Query().Get("model"); raw != "" {
		for _, m := range models {
			if strconv.FormatInt(m.ID, 10) == raw {
				d.Selected = m
				modelID = m.ID
				break
			}
		}
	}
	rows, err := s.st.RecentObservations(modelID, 30)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d.Rows = rows
	v := s.view(r, "Data harga", "dataharga")
	v.Data = d
	s.render(w, "dataharga", v)
}

func (s *Server) SimpanHarga(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer", "buyer")
	if !ok {
		return
	}
	_ = r.ParseForm()
	models, err := s.st.Models()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var sel store.Model
	for _, m := range models {
		if r.FormValue("model") == fmt.Sprintf("%d:%d", m.ID, m.VariantID) {
			sel = m
		}
	}
	grade := r.FormValue("grade")
	series := r.FormValue("series")
	source := r.FormValue("source")
	price := parseMoney(r.FormValue("price"))
	note := strings.TrimSpace(r.FormValue("note"))
	accs := r.Form["acc"]
	if sel.ID == 0 || !strings.Contains("ABCD", grade) || len(grade) != 1 ||
		(series != "sell" && series != "buy") || !validSource(source) || price <= 0 {
		http.Redirect(w, r, "/data-harga?err="+url.QueryEscape("Lengkapi model, grade, seri, sumber, dan harga."), http.StatusFound)
		return
	}
	id, err := s.st.AddObservation(store.NewObservation{
		ModelID: sel.ID, VariantID: sel.VariantID, Grade: grade, Series: series, Source: source,
		Price: price, Accessories: accs, ObservedAt: time.Now(), Note: note,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.st.Audit(u.Name, "tambah", "harga", id, "", rp(price)+" "+source)
	http.Redirect(w, r, "/data-harga?ok="+url.QueryEscape("Harga "+rp(price)+" tersimpan."), http.StatusFound)
}

func validSource(s string) bool {
	return s == "toko" || s == "pesaing" || s == "marketplace" || s == "penjual"
}

// ---------- Kas harian ----------

type kasData struct {
	Session   *store.CashSession
	Movements []store.CashMovement
	Expected  int64
	In        int64
	Out       int64
	Threshold int64
	Branch    string
	Branches  []store.Branch
	BranchID  int64
	IsOwner   bool
}

func (s *Server) kasData(r *http.Request) (*kasData, error) {
	u := userOf(r)
	branchID := branchOf(u)
	d := &kasData{}
	if u != nil {
		d.IsOwner = u.Role == "owner"
		d.Branch = u.Branch
	}
	if d.IsOwner {
		if bs, err := s.st.Branches(); err == nil {
			d.Branches = bs
		}
	}
	branchID = s.kasBranch(r)
	if branchID == 0 {
		if id, err := s.st.DefaultBranchID(); err == nil {
			branchID = id
		}
	}
	d.BranchID = branchID
	for _, b := range d.Branches {
		if b.ID == branchID {
			d.Branch = b.Name
		}
	}
	set, err := s.st.Settings()
	if err != nil {
		return nil, err
	}
	d.Threshold = set.CashThreshold
	sess, err := s.st.CurrentCash(branchID)
	if err == store.ErrNoCashSession {
		return d, nil
	}
	if err != nil {
		return nil, err
	}
	d.Session = &sess
	movs, err := s.st.CashMovements(sess.ID)
	if err != nil {
		return nil, err
	}
	d.Movements = movs
	expected := sess.Opening
	for _, m := range movs {
		switch {
		case m.Kind == "sale" || m.Kind == "in":
			if m.Method == "tunai" {
				d.In += m.Amount
			}
		case m.Kind == "expense" || m.Kind == "out" || m.Kind == "purchase":
			if m.Method == "tunai" {
				d.Out += m.Amount
			}
		}
	}
	expected += d.In - d.Out
	d.Expected = expected
	return d, nil
}

func (s *Server) Kas(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "kasir"); !ok {
		return
	}
	d, err := s.kasData(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(r, "Kas harian", "kas")
	v.Data = d
	s.render(w, "kas", v)
}

func (s *Server) BukaKas(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer", "kasir")
	if !ok {
		return
	}
	_ = r.ParseForm()
	opening := parseMoney(r.FormValue("opening"))
	branchID := s.kasBranch(r)
	if branchID == 0 {
		s.fail(w, r, fmt.Errorf("cabang tidak ditemukan"))
		return
	}
	if _, err := s.st.OpenCash(branchID, u.Name, opening); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&err=%s", branchID, url.QueryEscape(err.Error())), http.StatusFound)
		return
	}
	s.st.Audit(u.Name, "buka", "kas", 0, "", rp(opening))
	http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&ok=%s", branchID, url.QueryEscape("Kas dibuka dengan saldo awal "+rp(opening)+".")), http.StatusFound)
}

func (s *Server) TutupKas(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer", "kasir")
	if !ok {
		return
	}
	_ = r.ParseForm()
	d, err := s.kasData(r)
	if err != nil || d.Session == nil {
		http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&err=%s", d.BranchID, url.QueryEscape("Tidak ada sesi kas yang terbuka.")), http.StatusFound)
		return
	}
	counted := parseMoney(r.FormValue("counted"))
	expected, _, needsApproval, err := s.st.CloseCash(d.Session.ID, u.Name, counted, d.Threshold)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&err=%s", d.BranchID, url.QueryEscape(err.Error())), http.StatusFound)
		return
	}
	s.st.Audit(u.Name, "tutup", "kas", d.Session.ID, rp(expected), rp(counted))
	msg := "Kas ditutup. Selisih " + signedDiff(counted, expected) + "."
	if needsApproval {
		msg = "Selisih " + signedDiff(counted, expected) + " melebihi ambang. Menunggu persetujuan manajer."
		s.notify(fmt.Sprintf("Persetujuan kas %s: sistem %s, fisik %s, selisih %s. Oleh %s.",
			d.Branch, rp(expected), rp(counted), signedDiff(counted, expected), u.Name))
	}
	http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&ok=%s", d.BranchID, url.QueryEscape(msg)), http.StatusFound)
}

func (s *Server) CatatKas(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer", "kasir")
	if !ok {
		return
	}
	_ = r.ParseForm()
	d, err := s.kasData(r)
	if err != nil || d.Session == nil {
		http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&err=%s", d.BranchID, url.QueryEscape("Buka kas dulu.")), http.StatusFound)
		return
	}
	kind := r.FormValue("kind")
	if kind != "expense" && kind != "in" && kind != "out" {
		http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&err=%s", d.BranchID, url.QueryEscape("Jenis mutasi tidak valid.")), http.StatusFound)
		return
	}
	method := r.FormValue("method")
	if method != "tunai" && method != "transfer" && method != "qris" {
		method = "tunai"
	}
	amount := parseMoney(r.FormValue("amount"))
	note := strings.TrimSpace(r.FormValue("note"))
	if amount <= 0 || note == "" {
		http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&err=%s", d.BranchID, url.QueryEscape("Nominal dan keterangan wajib diisi.")), http.StatusFound)
		return
	}
	if err := s.st.AddCashMovement(d.Session.ID, d.Session.BranchID, kind, method, amount, note, u.Name); err != nil {
		s.fail(w, r, err)
		return
	}
	s.st.Audit(u.Name, "mutasi", "kas", d.Session.ID, "", kind+" "+rp(amount))
	http.Redirect(w, r, fmt.Sprintf("/kas?branch=%d&ok=%s", d.BranchID, url.QueryEscape("Mutasi kas tercatat.")), http.StatusFound)
}

// ---------- Pengaturan ----------

type pengaturanData struct {
	Settings          store.Settings
	Cats              []catRow
	MinMargin         string
	WeightToko        string
	WeightPesaing     string
	WeightMarketplace string
	WeightPenjual     string
	HalfLife          string
	Audit             []store.AuditRow
}

type catRow struct {
	Name  string
	Value string
}

func (s *Server) Pengaturan(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner"); !ok {
		return
	}
	set, err := s.st.Settings()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	cats, err := s.st.CategoryMargins()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	audit, err := s.st.AuditRecent(20)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := &pengaturanData{
		Settings:          set,
		Audit:             audit,
		MinMargin:         pctStr(set.MinMargin * 100),
		WeightToko:        pctStr(set.Weights["toko"]),
		WeightPesaing:     pctStr(set.Weights["pesaing"]),
		WeightMarketplace: pctStr(set.Weights["marketplace"]),
		WeightPenjual:     pctStr(set.Weights["penjual"]),
		HalfLife:          pctStr(set.HalfLifeDays),
	}
	for _, c := range cats {
		d.Cats = append(d.Cats, catRow{Name: c.Name, Value: pctStr(float64(c.Bp) / 100)})
	}
	v := s.view(r, "Pengaturan", "pengaturan")
	v.Data = d
	s.render(w, "pengaturan", v)
}

func (s *Server) SimpanPengaturan(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner")
	if !ok {
		return
	}
	_ = r.ParseForm()

	saveMargin := func(key, formKey string) {
		if raw := strings.TrimSpace(r.FormValue(formKey)); raw != "" {
			f, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
			if err == nil {
				_ = s.st.SaveSetting(key, strconv.Itoa(int(f*100+0.5)), u.Name)
			}
		}
	}
	saveWeight := func(key, formKey string) {
		if raw := strings.TrimSpace(r.FormValue(formKey)); raw != "" {
			f, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
			if err == nil {
				_ = s.st.SaveSetting(key, strconv.Itoa(int(f*10000+0.5)), u.Name)
			}
		}
	}
	saveInt := func(key, formKey string) {
		if raw := parseMoney(r.FormValue(formKey)); raw > 0 {
			_ = s.st.SaveSetting(key, strconv.FormatInt(raw, 10), u.Name)
		}
	}
	saveMargin("min_margin_bp", "min_margin")
	saveInt("est_cost", "est_cost")
	saveInt("period_days", "period_days")
	saveWeight("w_toko", "w_toko")
	saveWeight("w_pesaing", "w_pesaing")
	saveWeight("w_marketplace", "w_marketplace")
	saveWeight("w_penjual", "w_penjual")
	saveInt("cash_threshold", "cash_threshold")
	if raw := strings.TrimSpace(r.FormValue("half_life_days")); raw != "" {
		if f, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64); err == nil && f > 0 {
			_ = s.st.SaveSetting("half_life_days", strconv.Itoa(int(f+0.5)), u.Name)
		}
	}
	for _, c := range mustCategories(s) {
		raw := strings.TrimSpace(r.FormValue("margin_" + c))
		if raw == "" {
			continue
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
		if err != nil || f < 0 || f > 100 {
			continue
		}
		_ = s.st.SaveCategoryMargin(c, int(f*100+0.5), u.Name)
	}
	http.Redirect(w, r, "/pengaturan?ok="+url.QueryEscape("Pengaturan tersimpan dan tercatat di audit log."), http.StatusFound)
}

func mustCategories(s *Server) []string {
	cats, err := s.st.CategoryMargins()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, c.Name)
	}
	return out
}

// ---------- Nota ----------

func (s *Server) Nota(w http.ResponseWriter, r *http.Request) {
	u := userOf(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	d, err := s.st.SaleDetail(parseMoney(chi.URLParam(r, "id")))
	if err != nil {
		http.Error(w, "Nota tidak ditemukan", http.StatusNotFound)
		return
	}
	if branch := branchOf(u); branch != 0 && d.BranchID != branch {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages["nota"].ExecuteTemplate(w, "nota.html", map[string]any{"Sale": d, "Today": todayID(), "User": u}); err != nil {
		s.fail(w, r, err)
	}
}

func (s *Server) kasBranch(r *http.Request) int64 {
	u := userOf(r)
	if u == nil {
		return 0
	}
	if u.Role != "owner" {
		return u.BranchID
	}
	if v := r.FormValue("branch"); v != "" {
		return parseMoney(v)
	}
	if v := r.URL.Query().Get("branch"); v != "" {
		return parseMoney(v)
	}
	return 0
}

func (s *Server) NotaURL(id int64) string { return "/nota/" + strconv.FormatInt(id, 10) }

func pctStr(v float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(v, 'f', -1, 64), ".", ",")
}

var _ = url.Values{}
