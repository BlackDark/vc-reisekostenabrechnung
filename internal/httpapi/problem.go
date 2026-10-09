package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func writeProblemFields(w http.ResponseWriter, status int, code, title string, errs []api.FieldError) {
	p := api.Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Code:   code,
		Errors: &errs,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func writeProblem(w http.ResponseWriter, status int, code, title, detail string) {
	p := api.Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Code:   code,
	}
	if detail != "" {
		p.Detail = &detail
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	ct := r.Header.Get("Content-Type")
	if ct == "" || !hasPrefixFold(ct, "application/json") {
		writeProblem(w, http.StatusUnsupportedMediaType, "content_type", "Unsupported media type", "Content-Type must be application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		writeProblem(w, http.StatusBadRequest, "ungueltiger_body", "Invalid JSON", "")
		return false
	}
	return true
}

func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		a, b := s[i], prefix[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

func toNutzer(n sqlitedb.Nutzer) api.Nutzer {
	return api.Nutzer{
		Id:             n.ID,
		Anzeigename:    n.Anzeigename,
		Benutzername:   n.Benutzername,
		Email:          n.Email,
		IstAdmin:       n.IstAdminLokal || n.AdminUeberGruppe,
		KiErlaubt:      n.KiErlaubt,
		Personalnummer: n.Personalnummer,
		Sprache:        n.Sprache,
		Aktiv:          n.Aktiv,
		Version:        n.Version,
	}
}

func writeNutzer(w http.ResponseWriter, status int, n sqlitedb.Nutzer) {
	w.Header().Set("ETag", strconv.FormatInt(n.Version, 10))
	writeJSON(w, status, toNutzer(n))
}

func parseVersion(raw string) (int64, bool) {
	raw = trimQuotes(raw)
	if raw == "" || raw == "*" {
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func ifMatchVersion(header *string) (version int64, present, ok bool) {
	if header == nil || *header == "" {
		return 0, false, false
	}
	v, ok := parseVersion(*header)
	return v, true, ok
}
