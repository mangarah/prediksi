package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"prediksi/internal/store"
)

// ---------- Batal penjualan ----------

func (s *Server) VoidSale(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	_ = r.ParseForm()
	id := parseMoney(chi.URLParam(r, "id"))
	d, err := s.st.SaleDetail(id)
	if err != nil {
		http.Error(w, "Nota tidak ditemukan", http.StatusNotFound)
		return
	}
	if branch := branchOf(u); branch != 0 && d.BranchID != branch {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	if len(reason) < 3 {
		http.Redirect(w, r, "/nota/"+strconv.FormatInt(id, 10)+"?err="+url.QueryEscape("Alasan pembatalan wajib diisi."), http.StatusFound)
		return
	}
	if _, err := s.st.VoidSale(id, u.Name, reason); err != nil {
		http.Redirect(w, r, "/nota/"+strconv.FormatInt(id, 10)+"?err="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/nota/"+strconv.FormatInt(id, 10)+"?ok="+url.QueryEscape("Penjualan dibatalkan, unit kembali siap jual."), http.StatusFound)
}

// ---------- Detail unit ----------

func (s *Server) UnitLookup(w http.ResponseWriter, r *http.Request) {
	u := userOf(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	unit, err := s.st.UnitByCode(code)
	if err != nil {
		http.Redirect(w, r, homeFor(u.Role)+"?err="+url.QueryEscape("Unit "+code+" tidak ditemukan."), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/unit/"+strconv.FormatInt(unit.ID, 10), http.StatusFound)
}

func (s *Server) UnitPage(w http.ResponseWriter, r *http.Request) {
	u := userOf(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	d, err := s.st.UnitDetail(parseMoney(chi.URLParam(r, "id")))
	if err != nil {
		http.Error(w, "Unit tidak ditemukan", http.StatusNotFound)
		return
	}
	if branch := branchOf(u); branch != 0 && d.Unit.BranchID != branch {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}
	v := s.view(r, "Unit "+d.Unit.Code, "")
	v.Data = d
	s.render(w, "unit", v)
}

func (s *Server) SetUnitPrice(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	_ = r.ParseForm()
	id := parseMoney(chi.URLParam(r, "id"))
	price := parseMoney(r.FormValue("price"))
	if err := s.st.SetUnitPrice(id, price, u.Name); err != nil {
		http.Redirect(w, r, "/unit/"+strconv.FormatInt(id, 10)+"?err="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/unit/"+strconv.FormatInt(id, 10)+"?ok="+url.QueryEscape("Harga jual diperbarui."), http.StatusFound)
}

// ---------- Pengguna & cabang ----------

type usersData struct {
	Users    []store.UserRow
	Branches []store.Branch
}

func (s *Server) UsersPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner"); !ok {
		return
	}
	users, err := s.st.Users()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	branches, err := s.st.Branches()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(r, "Pengguna", "pengguna")
	v.Data = &usersData{Users: users, Branches: branches}
	s.render(w, "pengguna", v)
}

func (s *Server) CreateUser(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner")
	if !ok {
		return
	}
	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	role := r.FormValue("role")
	branchID := parseMoney(r.FormValue("branch"))
	if err := s.st.CreateUser(name, email, r.FormValue("password"), role, branchID); err != nil {
		s.redirectUsers(w, r, err.Error())
		return
	}
	s.st.Audit(u.Name, "tambah", "user", 0, "", email+" ("+role+")")
	s.redirectUsers(w, r, "")
}

func (s *Server) UpdateUser(w http.ResponseWriter, r *http.Request) {
	me, ok := s.allow(w, r, "owner")
	if !ok {
		return
	}
	_ = r.ParseForm()
	id := parseMoney(chi.URLParam(r, "id"))
	target, err := s.st.UserRowByID(id)
	if err != nil {
		s.redirectUsers(w, r, "Pengguna tidak ditemukan.")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	role := r.FormValue("role")
	branchID := parseMoney(r.FormValue("branch"))
	active := r.FormValue("active") == "1"
	if id == me.ID && (!active || role != me.Role) {
		s.redirectUsers(w, r, "Tidak bisa menonaktifkan atau mengubah peran akun sendiri.")
		return
	}
	if target.Role == "owner" && (role != "owner" || !active) {
		if n, _ := s.st.ActiveOwnerCount(); n <= 1 {
			s.redirectUsers(w, r, "Minimal harus ada satu owner aktif.")
			return
		}
	}
	if err := s.st.UpdateUser(id, name, email, role, branchID, active); err != nil {
		s.redirectUsers(w, r, err.Error())
		return
	}
	s.st.Audit(me.Name, "ubah", "user", id, target.Email+" ("+target.Role+")", email+" ("+role+")")
	s.redirectUsers(w, r, "")
}

func (s *Server) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	me, ok := s.allow(w, r, "owner")
	if !ok {
		return
	}
	_ = r.ParseForm()
	id := parseMoney(chi.URLParam(r, "id"))
	if err := s.st.SetUserPassword(id, r.FormValue("password")); err != nil {
		s.redirectUsers(w, r, err.Error())
		return
	}
	s.st.Audit(me.Name, "reset password", "user", id, "", "")
	s.redirectUsers(w, r, "")
}

func (s *Server) CreateBranch(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner")
	if !ok {
		return
	}
	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.redirectUsers(w, r, "Nama cabang wajib diisi.")
		return
	}
	if err := s.st.AddBranch(name); err != nil {
		s.redirectUsers(w, r, "Cabang gagal ditambah (mungkin sudah ada).")
		return
	}
	s.st.Audit(u.Name, "tambah", "cabang", 0, "", name)
	s.redirectUsers(w, r, "")
}

func (s *Server) redirectUsers(w http.ResponseWriter, r *http.Request, errMsg string) {
	target := "/pengguna"
	if errMsg != "" {
		target += "?err=" + url.QueryEscape(errMsg)
	} else {
		target += "?ok=" + url.QueryEscape("Perubahan tersimpan.")
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// ---------- Akun sendiri ----------

func (s *Server) Akun(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "Akun saya", "akun")
	s.render(w, "akun", v)
}

func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := userOf(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	_ = r.ParseForm()
	next := r.FormValue("password")
	if next != r.FormValue("confirm") {
		s.redirectAkun(w, r, "Konfirmasi password tidak sama.")
		return
	}
	if err := s.st.ChangePassword(u.ID, r.FormValue("current"), next); err != nil {
		s.redirectAkun(w, r, err.Error())
		return
	}
	if c, err := r.Cookie("sid"); err == nil {
		_ = s.st.DeleteOtherSessions(u.ID, c.Value)
	}
	s.st.Audit(u.Name, "ubah password", "user", u.ID, "", "")
	s.redirectAkun(w, r, "")
}

func (s *Server) redirectAkun(w http.ResponseWriter, r *http.Request, errMsg string) {
	target := "/akun"
	if errMsg != "" {
		target += "?err=" + url.QueryEscape(errMsg)
	} else {
		target += "?ok=" + url.QueryEscape("Password diperbarui. Perangkat lain sudah logout.")
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// ---------- Katalog ----------

type catalogData struct {
	Cats   []store.CatalogCategory
	Brands []store.CatalogBrand
	Models []store.CatalogModel
}

func (s *Server) CatalogPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.allow(w, r, "owner", "manajer"); !ok {
		return
	}
	cats, brands, models, err := s.st.Catalog()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(r, "Katalog", "katalog")
	v.Data = &catalogData{Cats: cats, Brands: brands, Models: models}
	s.render(w, "katalog", v)
}

func (s *Server) CatalogAction(w http.ResponseWriter, r *http.Request) {
	u, ok := s.allow(w, r, "owner", "manajer")
	if !ok {
		return
	}
	_ = r.ParseForm()
	var err error
	action := r.FormValue("action")
	switch action {
	case "kategori":
		err = s.st.AddCategory(r.FormValue("name"))
	case "merek":
		err = s.st.AddBrand(parseMoney(r.FormValue("category")), r.FormValue("name"))
	case "model":
		variants := strings.Split(r.FormValue("variants"), ",")
		err = s.st.AddModel(parseMoney(r.FormValue("brand")), r.FormValue("name"), r.FormValue("prefix"), variants)
	case "varian":
		err = s.st.AddVariant(parseMoney(r.FormValue("model")), r.FormValue("name"))
	default:
		err = store.ErrCatalogEmpty
	}
	if err != nil {
		http.Redirect(w, r, "/katalog?err="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	s.st.Audit(u.Name, "tambah", "katalog "+action, 0, "", strings.TrimSpace(r.FormValue("name")))
	http.Redirect(w, r, "/katalog?ok="+url.QueryEscape("Katalog diperbarui."), http.StatusFound)
}
