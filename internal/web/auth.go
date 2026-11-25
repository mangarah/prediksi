package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"prediksi/internal/store"
)

type ctxKey string

const userKey ctxKey = "user"

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("sid"); err == nil {
			if u, err := s.st.UserBySession(c.Value); err == nil {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, &u)))
				return
			}
		}
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
	})
}

func userOf(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

func (s *Server) branchScope(r *http.Request) int64 {
	u := userOf(r)
	if u == nil || u.Role == "owner" {
		return 0
	}
	return u.BranchID
}

func (s *Server) allow(w http.ResponseWriter, r *http.Request, roles ...string) (*store.User, bool) {
	u := userOf(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil, false
	}
	for _, role := range roles {
		if u.Role == role {
			return u, true
		}
	}
	http.Error(w, "Akses ditolak untuk peran "+u.Role, http.StatusForbidden)
	return nil, false
}

func (s *Server) LoginPage(w http.ResponseWriter, r *http.Request) {
	if userOf(r) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	if c, err := r.Cookie("sid"); err == nil {
		if _, err := s.st.UserBySession(c.Value); err == nil {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.login.ExecuteTemplate(w, "login.html", map[string]string{"Error": r.URL.Query().Get("err"), "Email": r.URL.Query().Get("email")}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type loginAttempt struct {
	count int
	until time.Time
}

func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = r.Header.Get("Referer")
			}
			if origin != "" {
				if u, err := url.Parse(origin); err == nil && u.Host != "" && u.Host != r.Host {
					http.Error(w, "Permintaan ditolak", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) loginBlocked(key string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	a, ok := s.loginFails[key]
	if !ok {
		return false
	}
	if time.Now().After(a.until) {
		delete(s.loginFails, key)
		return false
	}
	return a.count >= 5
}

func (s *Server) loginFailed(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if len(s.loginFails) > 5000 {
		now := time.Now()
		for k, a := range s.loginFails {
			if now.After(a.until) {
				delete(s.loginFails, k)
			}
		}
	}
	a := s.loginFails[key]
	a.count++
	a.until = time.Now().Add(10 * time.Minute)
	s.loginFails[key] = a
}

func (s *Server) loginOK(key string) {
	s.loginMu.Lock()
	delete(s.loginFails, key)
	s.loginMu.Unlock()
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	pass := r.FormValue("password")
	key := clientIP(r) + "|" + email
	if s.loginBlocked(key) {
		http.Redirect(w, r, "/login?err="+url.QueryEscape("Terlalu banyak percobaan. Coba lagi dalam 10 menit."), http.StatusFound)
		return
	}
	u, hash, err := s.st.UserByEmail(email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) != nil {
		s.loginFailed(key)
		http.Redirect(w, r, "/login?err="+url.QueryEscape("Email atau password salah.")+"&email="+url.QueryEscape(email), http.StatusFound)
		return
	}
	s.loginOK(key)

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		http.Error(w, "gagal membuat sesi", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(buf)
	if err := s.st.CreateSession(u.ID, token, time.Now().Add(12*time.Hour)); err != nil {
		http.Error(w, "gagal membuat sesi", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "sid", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Expires: time.Now().Add(12 * time.Hour),
	})
	s.st.Audit(u.Name, "login", "user", u.ID, "", "")
	http.Redirect(w, r, homeFor(u.Role), http.StatusFound)
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("sid"); err == nil {
		_ = s.st.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "sid", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func homeFor(role string) string {
	switch role {
	case "kasir":
		return "/kasir"
	case "buyer":
		return "/cek-harga"
	default:
		return "/dashboard"
	}
}
