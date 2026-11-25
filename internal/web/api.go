package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func (s *Server) apiAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := os.Getenv("API_KEY")
		if key == "" {
			key = "dev-key"
		}
		if r.Header.Get("X-API-Key") != key {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "API key salah"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) APIQuote(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	v := url.Values{}
	modelID, variantID := q.Get("model"), q.Get("variant")
	if modelID != "" && variantID != "" {
		v.Set("model", modelID+":"+variantID)
	}
	v.Set("grade", q.Get("grade"))
	v.Set("deal", q.Get("deal"))
	if acc := q.Get("acc"); acc != "" {
		v.Set("acc_submitted", "1")
		for _, a := range strings.Split(acc, ",") {
			v.Add("acc", a)
		}
	}
	qv, err := s.buildQuote(v)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"model": qv.Selected.Name, "variant": qv.Selected.Variant, "grade": qv.Grade,
		"market": qv.Q.Market, "buy_limit": qv.Q.BuyLimit, "recommended": qv.Q.Recommended,
		"min_price": qv.Q.MinPrice, "p25": qv.Q.P25, "p75": qv.Q.P75,
		"count": qv.Q.Count, "outliers": len(qv.Q.Outliers), "low_data": qv.Q.LowData,
	})
}

func (s *Server) APIUnits(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	branch := parseMoney(q.Get("branch"))
	limit := int(parseMoney(q.Get("limit")))
	units, err := s.st.UnitsByStatus(q.Get("status"), branch, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type item struct {
		Code     string `json:"code"`
		Model    string `json:"model"`
		Variant  string `json:"variant"`
		Grade    string `json:"grade"`
		Branch   string `json:"branch"`
		Status   string `json:"status"`
		Price    int64  `json:"price"`
		MinPrice int64  `json:"min_price"`
		AgeDays  int    `json:"age_days"`
	}
	out := make([]item, 0, len(units))
	for _, u := range units {
		out = append(out, item{u.Code, u.Model, u.Variant, u.Grade, u.Branch, u.Status, u.ListedPrice, u.MinPrice, u.AgeDays})
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(out), "units": out})
}

func (s *Server) APIDemand(w http.ResponseWriter, r *http.Request) {
	branch := parseMoney(r.URL.Query().Get("branch"))
	rows, err := s.st.DemandRows(30, branch)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if len(rows) > 20 {
		rows = rows[:20]
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(rows), "models": rows})
}
