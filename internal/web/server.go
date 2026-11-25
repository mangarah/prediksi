package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"prediksi/internal/notify"
	"prediksi/internal/pricing"
	"prediksi/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	st         *store.Store
	pages      map[string]*template.Template
	login      *template.Template
	tg         *notify.Telegram
	loginMu    sync.Mutex
	loginFails map[string]loginAttempt
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"rp":           rp,
		"num":          thousands,
		"rpShort":      rpShort,
		"gradeLabel":   gradeLabel,
		"statusLabel":  statusLabel,
		"statusPill":   statusPill,
		"methodLabel":  methodLabel,
		"kindLabel":    kindLabel,
		"kindPill":     kindPill,
		"kindAccent":   kindAccent,
		"amountLabel":  amountLabel,
		"compareLabel": compareLabel,
		"initials":     initials,
		"sourceLabel":  sourceLabel,
		"signedDiff":   signedDiff,
		"profitPct":    profitPct,
		"urlquery":     url.QueryEscape,
	}
}

func New(st *store.Store, tg *notify.Telegram) (*Server, error) {
	funcs := templateFuncs()
	s := &Server{st: st, pages: map[string]*template.Template{}, tg: tg, loginFails: map[string]loginAttempt{}}

	login, err := template.New("login").Funcs(funcs).ParseFS(templateFS, "templates/login.html")
	if err != nil {
		return nil, fmt.Errorf("parse login: %w", err)
	}
	s.login = login

	for _, name := range []string{"dashboard", "cekharga", "kasir", "persetujuan", "pembelian", "dataharga", "kas", "pengaturan", "tukartambah", "transfer", "laporan", "demand", "opname", "impor", "unit", "pengguna", "akun", "katalog"} {
		files := []string{"templates/layout.html", "templates/partials.html", "templates/" + name + ".html"}
		t, err := template.New("layout").Funcs(funcs).ParseFS(templateFS, files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		s.pages[name] = t
	}
	nota, err := template.New("nota").Funcs(funcs).ParseFS(templateFS, "templates/nota.html")
	if err != nil {
		return nil, fmt.Errorf("parse nota: %w", err)
	}
	s.pages["nota"] = nota
	label, err := template.New("label").Funcs(funcs).ParseFS(templateFS, "templates/label.html")
	if err != nil {
		return nil, fmt.Errorf("parse label: %w", err)
	}
	s.pages["label"] = label
	return s, nil
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(s.sameOrigin)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := s.st.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "db tidak tersedia", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/login", s.LoginPage)
	r.Post("/login", s.Login)
	r.Post("/logout", s.Logout)

	r.Group(func(pr chi.Router) {
		pr.Use(s.auth)
		pr.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, homeFor(userOf(r).Role), http.StatusFound)
		})
		pr.Get("/dashboard", s.Dashboard)
		pr.Get("/cek-harga", s.CekHarga)
		pr.Get("/cek-harga/quote", s.QuotePartial)
		pr.Get("/kasir", s.Kasir)
		pr.Post("/kasir/jual", s.Jual)
		pr.Get("/kasir/ringkasan", s.RingkasanPartial)
		pr.Get("/persetujuan", s.Persetujuan)
		pr.Post("/persetujuan/{id}/setujui", s.Setujui)
		pr.Post("/persetujuan/{id}/tolak", s.Tolak)
		pr.Get("/pembelian", s.Pembelian)
		pr.Post("/pembelian", s.CatatPembelian)
		pr.Get("/pembelian/preview", s.PembelianPreview)
		pr.Get("/data-harga", s.DataHarga)
		pr.Post("/data-harga", s.SimpanHarga)
		pr.Get("/kas", s.Kas)
		pr.Post("/kas/buka", s.BukaKas)
		pr.Post("/kas/tutup", s.TutupKas)
		pr.Post("/kas/movement", s.CatatKas)
		pr.Get("/pengaturan", s.Pengaturan)
		pr.Post("/pengaturan", s.SimpanPengaturan)
		pr.Get("/nota/{id}", s.Nota)
		pr.Post("/nota/{id}/batal", s.VoidSale)
		pr.Get("/unit", s.UnitLookup)
		pr.Get("/unit/{id}", s.UnitPage)
		pr.Post("/unit/{id}/harga", s.SetUnitPrice)
		pr.Get("/pengguna", s.UsersPage)
		pr.Post("/pengguna", s.CreateUser)
		pr.Post("/pengguna/cabang", s.CreateBranch)
		pr.Post("/pengguna/{id}", s.UpdateUser)
		pr.Post("/pengguna/{id}/password", s.ResetUserPassword)
		pr.Get("/akun", s.Akun)
		pr.Post("/akun/password", s.ChangePassword)
		pr.Get("/katalog", s.CatalogPage)
		pr.Post("/katalog", s.CatalogAction)
		pr.Get("/tukar-tambah", s.TukarTambah)
		pr.Post("/tukar-tambah", s.SimpanTukarTambah)
		pr.Get("/transfer", s.TransferPage)
		pr.Post("/transfer/kirim", s.KirimTransfer)
		pr.Post("/transfer/{id}/terima", s.TerimaTransfer)
		pr.Get("/laporan", s.Laporan)
		pr.Get("/laporan/export", s.LaporanCSV)
		pr.Get("/prediksi", s.Demand)
		pr.Get("/opname", s.Opname)
		pr.Post("/opname", s.Opname)
		pr.Get("/impor", s.Impor)
		pr.Post("/impor", s.Impor)
		pr.Get("/label/{id}", s.Label)
		pr.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("data/uploads"))))
	})

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(s.apiAuth)
		api.Get("/quote", s.APIQuote)
		api.Get("/units", s.APIUnits)
		api.Get("/demand", s.APIDemand)
	})
	return r
}

type View struct {
	Title      string
	Nav        string
	Today      string
	Query      string
	Pending    int
	Message    string
	Error      string
	User       *store.User
	ShowProfit bool
	Data       any
}

func (s *Server) view(r *http.Request, title, nav string) *View {
	q := r.URL.Query()
	u := userOf(r)
	v := &View{
		Title:   title,
		Nav:     nav,
		Today:   todayID(),
		Query:   q.Get("q"),
		Message: q.Get("ok"),
		Error:   q.Get("err"),
		User:    u,
	}
	if u != nil {
		v.ShowProfit = u.Role == "owner" || u.Role == "manajer"
		if n, err := s.st.CountApprovals("menunggu", s.branchScope(r)); err == nil {
			v.Pending = n
		}
	}
	return v
}

func (s *Server) render(w http.ResponseWriter, page string, v *View) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[page].ExecuteTemplate(w, "layout", v); err != nil {
		log.Printf("render %s: %v", page, err)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, page, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[page].ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render partial %s/%s: %v", page, name, err)
	}
}

// ---------- Dashboard ----------

type dashboardData struct {
	KPIs     store.KPIs
	Days     []store.DayPoint
	DayMax   int64
	DayQ1    int64
	DayQ2    int64
	DayQ3    int64
	Cats     []catSeg
	CatTotal int
	Units    []store.Unit
}

type catSeg struct {
	store.CatSlice
	Dash   string
	Rest   string
	Offset string
}

func (s *Server) Dashboard(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	branch := branchOf(u)

	kpis, err := s.st.KPIs(branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	days, err := s.st.Sales7Days(branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	cats, err := s.st.SalesByCategory(30, branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	units, err := s.st.RecentUnits(9, branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	d := &dashboardData{KPIs: kpis, Days: days, Units: units}
	for _, p := range days {
		if p.Total > d.DayMax {
			d.DayMax = p.Total
		}
	}
	d.DayQ3, d.DayQ2, d.DayQ1 = d.DayMax*3/4, d.DayMax/2, d.DayMax/4

	const circ = 339.292
	var total int
	for _, c := range cats {
		total += c.Units
	}
	var arcSum float64
	for _, c := range cats {
		arc := 0.0
		if total > 0 {
			arc = circ * float64(c.Units) / float64(total)
		}
		d.Cats = append(d.Cats, catSeg{CatSlice: c, Dash: f2(arc), Rest: f2(circ - arc), Offset: f2(-arcSum)})
		arcSum += arc
	}
	d.CatTotal = total

	v := s.view(r, "Dashboard", "dashboard")
	v.Data = d
	v.Pending = kpis.Pending
	s.render(w, "dashboard", v)
}

// ---------- Cek harga ----------

type quoteView struct {
	Models       []store.Model
	Selected     store.Model
	Grades       []string
	Grade        string
	Accessories  []store.Accessory
	Checked      map[string]bool
	Deal         int64
	Q            pricing.Result
	TargetPct    int
	EstCost      int64
	PeriodDays   int
	ModalCost    int64
	FullSet      bool
	Observations []obsView
}

type obsView struct {
	SourceLabel string
	Price       int64
	Grade       string
	Date        string
	Weight      string
	Outlier     bool
	Buy         bool
}

var gradeCodes = []string{"A", "B", "C", "D"}

func (s *Server) buildQuote(v url.Values) (*quoteView, error) {
	models, err := s.st.Models()
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("katalog kosong")
	}
	qv := &quoteView{Models: models, Grades: gradeCodes, Selected: models[0], Grade: "A"}
	if raw := v.Get("model"); raw != "" {
		for _, m := range models {
			if fmt.Sprintf("%d:%d", m.ID, m.VariantID) == raw {
				qv.Selected = m
				break
			}
		}
	}
	if g := v.Get("grade"); g != "" && strings.Contains("ABCD", g) && len(g) == 1 {
		qv.Grade = g
	}

	accs, err := s.st.Accessories()
	if err != nil {
		return nil, err
	}
	qv.Accessories = accs

	checked := map[string]bool{}
	if v.Get("acc_submitted") == "" {
		for _, a := range accs {
			checked[a.Code] = true
		}
		qv.FullSet = true
	} else {
		for _, code := range v["acc"] {
			checked[code] = true
		}
	}
	for _, a := range accs {
		if !checked[a.Code] {
			qv.FullSet = false
		}
	}
	qv.Checked = checked

	set, err := s.st.Settings()
	if err != nil {
		return nil, err
	}
	obs, err := s.st.Observations(qv.Selected.ID, qv.Selected.VariantID, set.PeriodDays)
	if err != nil {
		return nil, err
	}

	var deduction int64
	for _, a := range accs {
		if !checked[a.Code] {
			deduction += a.Deduction
		}
	}
	target := set.TargetMargin[qv.Selected.Category]
	if target <= 0 {
		target = 0.12
	}
	deal := parseMoney(v.Get("deal"))
	qv.Deal = deal
	qv.TargetPct = int(target*100 + 0.5)
	qv.EstCost = set.EstCost
	qv.PeriodDays = set.PeriodDays

	params := pricing.Params{
		TargetMargin:       target,
		MinMargin:          set.MinMargin,
		EstCost:            set.EstCost,
		GradeFactor:        set.GradeFactors[qv.Grade],
		AccessoryDeduction: deduction,
		Weights:            set.Weights,
		HalfLifeDays:       set.HalfLifeDays,
		Now:                time.Now(),
	}
	if deal > 0 {
		params.Cost = deal + set.EstCost
	}
	qv.Q = pricing.Quote(obs, qv.Grade, params)
	if deal > 0 {
		qv.ModalCost = deal + set.EstCost
	}

	outliers := map[string]bool{}
	for _, o := range qv.Q.Outliers {
		outliers[obsKey(o.Price, o.ObservedAt)] = true
	}
	for _, o := range obs {
		ov := obsView{
			SourceLabel: sourceLabel(o.Source),
			Price:       o.Price,
			Grade:       o.Grade,
			Date:        o.ObservedAt.Format("2 Jan 2006"),
			Weight:      "—",
			Buy:         o.Series == pricing.SeriesBuy,
		}
		if !ov.Buy {
			ov.Outlier = outliers[obsKey(o.Price, o.ObservedAt)]
			if !ov.Outlier {
				ov.Weight = strings.Replace(fmt.Sprintf("%.2f", pricing.WeightOf(o, params)), ".", ",", 1)
			}
		}
		qv.Observations = append(qv.Observations, ov)
	}
	return qv, nil
}

func (s *Server) CekHarga(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "buyer", "kasir"); !ok {
		return
	}
	qv, err := s.buildQuote(r.URL.Query())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(r, "Cek harga", "cekharga")
	v.Data = qv
	s.render(w, "cekharga", v)
}

func (s *Server) QuotePartial(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "buyer", "kasir"); !ok {
		return
	}
	qv, err := s.buildQuote(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderPartial(w, "cekharga", "quote_result", qv)
}

// ---------- Kasir ----------

type kasirData struct {
	Units      []store.Unit
	Selected   *store.Unit
	Price      int64
	Discount   int64
	Total      int64
	Cost       int64
	Profit     int64
	MarginPct  int
	BelowMin   bool
	Method     string
	ShowProfit bool
	Sales      []store.SaleRow
}

func (s *Server) kasirData(r *http.Request, unitID, price, discount int64) (*kasirData, error) {
	branch := s.branchScope(r)
	units, err := s.st.ReadyUnits(branch)
	if err != nil {
		return nil, err
	}
	d := &kasirData{Units: units, Method: "tunai"}
	if u := userOf(r); u != nil {
		d.ShowProfit = u.Role == "owner" || u.Role == "manajer"
	}
	if unitID == 0 && len(units) > 0 {
		unitID = units[0].ID
	}
	if unitID > 0 {
		u, err := s.st.UnitByID(unitID)
		if err != nil {
			return nil, err
		}
		if branch != 0 && u.BranchID != branch {
			return nil, fmt.Errorf("unit milik cabang lain")
		}
		if u.Status == "siap" {
			d.Selected = &u
		}
	}
	if d.Selected != nil {
		if price == 0 {
			price = d.Selected.ListedPrice
		}
		d.Price = price
		d.Discount = discount
		d.Cost = d.Selected.Cost
		d.Total = price - discount
		if d.Total < 0 {
			d.Total = 0
		}
		d.Profit = d.Total - d.Cost
		if d.Cost > 0 {
			d.MarginPct = int(d.Profit * 100 / d.Cost)
		}
		d.BelowMin = price < d.Selected.MinPrice
	}
	sales, err := s.st.SalesToday(branch)
	if err != nil {
		return nil, err
	}
	d.Sales = sales
	return d, nil
}

func (s *Server) Kasir(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "kasir"); !ok {
		return
	}
	q := r.URL.Query()
	d, err := s.kasirData(r, parseMoney(q.Get("unit")), parseMoney(q.Get("price")), parseMoney(q.Get("discount")))
	if err != nil {
		if strings.Contains(err.Error(), "cabang lain") {
			http.Error(w, "Akses ditolak", http.StatusForbidden)
			return
		}
		s.fail(w, r, err)
		return
	}
	v := s.view(r, "Kasir", "kasir")
	v.Data = d
	s.render(w, "kasir", v)
}

func (s *Server) RingkasanPartial(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "kasir"); !ok {
		return
	}
	q := r.URL.Query()
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		q = r.Form
	}
	d, err := s.kasirData(r, parseMoney(q.Get("unit_id")), parseMoney(q.Get("price")), parseMoney(q.Get("discount")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderPartial(w, "kasir", "ringkasan", d)
}

func (s *Server) Jual(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer", "kasir")
	if !ok {
		return
	}
	_ = r.ParseForm()
	unitID := parseMoney(r.FormValue("unit_id"))
	price := parseMoney(r.FormValue("price"))
	discount := parseMoney(r.FormValue("discount"))
	method := r.FormValue("method")
	if method != "tunai" && method != "transfer" && method != "qris" {
		method = "tunai"
	}
	warranty := int(parseMoney(r.FormValue("warranty")))
	back := fmt.Sprintf("/kasir?unit=%d", unitID)
	if price <= 0 {
		http.Redirect(w, r, back+"&err="+url.QueryEscape("Harga jual wajib diisi."), http.StatusFound)
		return
	}
	unit, err := s.st.UnitByID(unitID)
	if err == nil {
		if branch := branchOf(u); branch != 0 && unit.BranchID != branch {
			http.Error(w, "Akses ditolak", http.StatusForbidden)
			return
		}
		if price < unit.MinPrice {
			title := fmt.Sprintf("%s %s · Grade %s", unit.Model, unit.Variant, unit.Grade)
			if err := s.st.CreatePriceApproval(unit.ID, unit.BranchID, price, unit.MinPrice, u.Name, title); err != nil {
				log.Printf("price approval: %v", err)
				http.Redirect(w, r, back+"&err="+url.QueryEscape("Gagal membuat permintaan persetujuan."), http.StatusFound)
				return
			}
			s.st.Audit(u.Name, "minta persetujuan harga", "unit", unit.ID, rp(unit.MinPrice), rp(price))
			s.notify(fmt.Sprintf("Persetujuan harga: %s dijual %s (minimal %s). Oleh %s.", unit.Code, rp(price), rp(unit.MinPrice), u.Name))
			msg := fmt.Sprintf("Harga %s di bawah minimal %s. Permintaan persetujuan dibuat; transaksi belum tersimpan.", rp(price), rp(unit.MinPrice))
			http.Redirect(w, r, back+"&err="+url.QueryEscape(msg), http.StatusFound)
			return
		}
	}
	invoice, profit, err := s.st.CreateSale(store.SaleInput{
		UnitID: unitID, Price: price, Discount: discount, Method: method,
		BuyerName: r.FormValue("buyer"), Warranty: warranty, By: u.Name,
	})
	switch {
	case err == store.ErrUnitNotReady:
		http.Redirect(w, r, back+"&err="+url.QueryEscape("Unit tidak siap dijual (mungkin sudah terjual)."), http.StatusFound)
	case err == store.ErrBelowMin:
		http.Redirect(w, r, back+"&err="+url.QueryEscape("Harga di bawah harga minimal."), http.StatusFound)
	case err != nil:
		log.Printf("sale: %v", err)
		http.Redirect(w, r, back+"&err="+url.QueryEscape("Gagal menyimpan penjualan."), http.StatusFound)
	default:
		s.st.Audit(u.Name, "sale", "sale", 0, "", invoice)
		msg := fmt.Sprintf("Penjualan %s tersimpan. Laba %s.", invoice, rp(profit))
		http.Redirect(w, r, back+"&ok="+url.QueryEscape(msg), http.StatusFound)
	}
}

// ---------- Persetujuan ----------

type approvalData struct {
	Pending       []store.Approval
	PendingCount  int
	ApprovedToday int
	RejectedToday int
}

func (s *Server) Persetujuan(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	branch := branchOf(u)
	pending, err := s.st.Approvals("menunggu", 20, branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	approved, _ := s.st.DecidedToday("disetujui", branch)
	rejected, _ := s.st.DecidedToday("ditolak", branch)
	d := &approvalData{Pending: pending, PendingCount: len(pending), ApprovedToday: approved, RejectedToday: rejected}
	v := s.view(r, "Persetujuan", "persetujuan")
	v.Data = d
	s.render(w, "persetujuan", v)
}

func (s *Server) Setujui(w http.ResponseWriter, r *http.Request) { s.decide(w, r, true) }
func (s *Server) Tolak(w http.ResponseWriter, r *http.Request)   { s.decide(w, r, false) }

func (s *Server) decide(w http.ResponseWriter, r *http.Request, approve bool) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	id := parseMoney(chi.URLParam(r, "id"))
	if err := s.st.DecideApproval(id, approve, u.Name, branchOf(u)); err != nil {
		http.Redirect(w, r, "/persetujuan?err="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	action := "tolak"
	msg := "Persetujuan ditolak. Unit dibatalkan."
	if approve {
		action, msg = "setujui", "Persetujuan diterima."
	}
	s.st.Audit(u.Name, action, "approval", id, "", "")
	http.Redirect(w, r, "/persetujuan?ok="+url.QueryEscape(msg), http.StatusFound)
}

// ---------- Pembelian ----------

type pembelianData struct {
	*quoteView
	NeedsApproval bool
	Profit        int64
}

func (s *Server) Pembelian(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "buyer"); !ok {
		return
	}
	qv, err := s.buildQuote(r.URL.Query())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := &pembelianData{quoteView: qv}
	s.fillPembelian(d)
	v := s.view(r, "Catat pembelian", "pembelian")
	v.Data = d
	s.render(w, "pembelian", v)
}

func (s *Server) fillPembelian(d *pembelianData) {
	if d.Deal > 0 {
		d.NeedsApproval = d.Q.LowData || d.Deal > d.Q.BuyLimit
		d.Profit = d.Q.Recommended - d.ModalCost
	}
}

func (s *Server) PembelianPreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer", "buyer"); !ok {
		return
	}
	qv, err := s.buildQuote(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d := &pembelianData{quoteView: qv}
	s.fillPembelian(d)
	s.renderPartial(w, "pembelian", "purchase_summary", d)
}

func (s *Server) CatatPembelian(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer", "buyer")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	_ = r.ParseForm()
	_ = r.ParseMultipartForm(9 << 20)
	qv, err := s.buildQuote(r.Form)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	photo, err := savePhoto(r)
	if err != nil {
		http.Redirect(w, r, "/pembelian?err="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	deal := qv.Deal
	seller := strings.TrimSpace(r.FormValue("seller"))
	if deal <= 0 || seller == "" {
		http.Redirect(w, r, "/pembelian?err="+url.QueryEscape("Harga deal dan nama penjual wajib diisi."), http.StatusFound)
		return
	}
	branchID := branchOf(u)
	if branchID == 0 {
		if branchID, err = s.st.DefaultBranchID(); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	method := r.FormValue("method")
	if method != "tunai" && method != "transfer" && method != "qris" {
		method = "tunai"
	}

	buyLimit := qv.Q.BuyLimit
	if qv.Q.LowData {
		buyLimit = 0
	}
	pending := deal > buyLimit
	var accs []string
	for _, a := range qv.Accessories {
		if qv.Checked[a.Code] {
			accs = append(accs, a.Code)
		}
	}
	unitID, err := s.st.CreatePurchase(store.PurchaseInput{
		BranchID:    branchID,
		ModelID:     qv.Selected.ID,
		VariantID:   qv.Selected.VariantID,
		Prefix:      qv.Selected.Prefix,
		Title:       fmt.Sprintf("%s %s · Grade %s", qv.Selected.Name, qv.Selected.Variant, qv.Grade),
		Grade:       qv.Grade,
		SellerName:  seller,
		SellerPhone: strings.TrimSpace(r.FormValue("phone")),
		Serial:      strings.TrimSpace(r.FormValue("serial")),
		Accessories: accs,
		DealPrice:   deal,
		EstCost:     qv.EstCost,
		Method:      method,
		Recommended: qv.Q.Recommended,
		MinPrice:    qv.Q.MinPrice,
		BuyLimit:    buyLimit,
		Requester:   u.Name,
		Photo:       photo,
		Override:    r.FormValue("override") == "1",
		OverrideWhy: strings.TrimSpace(r.FormValue("override_reason")),
	})
	if err == store.ErrDuplicateSerial {
		http.Redirect(w, r, "/pembelian?err="+url.QueryEscape("Nomor seri sudah pernah masuk. Centang 'izinkan seri ganda' dan isi alasan untuk lanjut."), http.StatusFound)
		return
	}
	if err != nil {
		log.Printf("purchase: %v", err)
		http.Redirect(w, r, "/pembelian?err="+url.QueryEscape("Gagal menyimpan pembelian."), http.StatusFound)
		return
	}
	s.st.Audit(u.Name, "purchase", "unit", unitID, "", fmt.Sprintf("deal %d", deal))
	if r.FormValue("override") == "1" {
		s.notify(fmt.Sprintf("Nomor seri ganda diizinkan: %s %s, unit %d. Alasan: %s. Oleh %s.",
			qv.Selected.Name, qv.Selected.Variant, unitID, r.FormValue("override_reason"), u.Name))
	}
	msg := fmt.Sprintf("Unit %d tercatat dan siap dijual.", unitID)
	if pending {
		msg = fmt.Sprintf("Unit %d tercatat, menunggu persetujuan karena di atas batas beli %s.", unitID, rp(buyLimit))
		s.notify(fmt.Sprintf("Persetujuan pembelian: %s %s, deal %s (batas %s). Oleh %s.",
			qv.Selected.Name, qv.Selected.Variant, rp(deal), rp(buyLimit), u.Name))
	}
	http.Redirect(w, r, "/pembelian?ok="+url.QueryEscape(msg), http.StatusFound)
}

func (s *Server) notify(text string) {
	if s.tg == nil || !s.tg.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.tg.Send(ctx, text); err != nil {
			log.Printf("telegram: %v", err)
		}
	}()
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	http.Error(w, "Terjadi kesalahan: "+err.Error(), http.StatusInternalServerError)
}

func branchOf(u *store.User) int64 {
	if u == nil || u.Role == "owner" {
		return 0
	}
	return u.BranchID
}

// ---------- helpers ----------

func rp(v int64) string { return "Rp " + thousands(v) }

func thousands(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatInt(v, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "−" + b.String()
	}
	return b.String()
}

func rpShort(v int64) string {
	switch {
	case v >= 1_000_000:
		return fmt.Sprintf("Rp %s jt", strings.Replace(fmt.Sprintf("%.1f", float64(v)/1_000_000), ".", ",", 1))
	case v >= 1_000:
		return fmt.Sprintf("Rp %s rb", strings.Replace(fmt.Sprintf("%.0f", float64(v)/1_000), ".", ",", 1))
	default:
		return "Rp 0"
	}
}

func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func parseMoney(s string) int64 {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	n, _ := strconv.ParseInt(b.String(), 10, 64)
	return n
}

func todayID() string {
	t := time.Now()
	days := [...]string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}
	months := [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	return fmt.Sprintf("%s, %d %s %d", days[t.Weekday()], t.Day(), months[t.Month()-1], t.Year())
}

func obsKey(price int64, at time.Time) string {
	return fmt.Sprintf("%d|%d", price, at.Unix())
}

func sourceLabel(s string) string {
	switch s {
	case "toko":
		return "Penjualan toko"
	case "pesaing":
		return "Toko pesaing"
	case "marketplace":
		return "Marketplace"
	case "penjual":
		return "Tawaran penjual"
	default:
		return s
	}
}

func gradeLabel(code string) string {
	switch code {
	case "A":
		return "A — mulus"
	case "B":
		return "B — lecet ringan"
	case "C":
		return "C — ada minus"
	case "D":
		return "D — rusak"
	default:
		return code
	}
}

func statusLabel(s string) string {
	switch s {
	case "siap":
		return "siap jual"
	case "servis":
		return "servis"
	case "dicek":
		return "dicek"
	case "menunggu":
		return "menunggu"
	case "terjual":
		return "terjual"
	case "retur":
		return "retur"
	default:
		return s
	}
}

func statusPill(s string) string {
	switch s {
	case "siap":
		return "bg-emerald-50 text-emerald-700"
	case "servis":
		return "bg-amber-50 text-amber-700"
	case "menunggu":
		return "bg-rose-50 text-rose-700"
	case "terjual":
		return "bg-slate-100 text-slate-600"
	default:
		return "bg-slate-100 text-slate-600"
	}
}

func methodLabel(m string) string {
	switch m {
	case "tunai":
		return "Tunai"
	case "transfer":
		return "Transfer"
	case "qris":
		return "QRIS"
	default:
		return m
	}
}

func kindLabel(k string) string {
	switch k {
	case "purchase":
		return "di atas batas beli"
	case "price":
		return "di bawah harga minimal"
	case "cash":
		return "selisih tutup kas"
	default:
		return k
	}
}

func kindPill(k string) string {
	switch k {
	case "purchase":
		return "bg-amber-50 text-amber-700"
	case "price":
		return "bg-rose-50 text-rose-700"
	default:
		return "bg-indigo-50 text-indigo-700"
	}
}

func kindAccent(k string) string {
	switch k {
	case "purchase":
		return "bg-amber-400"
	case "price":
		return "bg-rose-400"
	default:
		return "bg-indigo-400"
	}
}

func amountLabel(k string) string {
	if k == "cash" {
		return "Saldo sistem"
	}
	return "Nilai"
}

func compareLabel(k string) string {
	switch k {
	case "purchase":
		return "Batas beli"
	case "price":
		return "Harga minimal"
	case "cash":
		return "Hitung fisik"
	default:
		return "Pembanding"
	}
}

func initials(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	return strings.ToUpper(name[:1])
}

func signedDiff(a, b int64) string {
	if a > b {
		return "+ " + rp(a-b)
	}
	if a < b {
		return "− " + rp(b-a)
	}
	return "Rp 0"
}

func profitPct(profit, cost int64) int {
	if cost <= 0 {
		return 0
	}
	return int(profit * 100 / cost)
}
