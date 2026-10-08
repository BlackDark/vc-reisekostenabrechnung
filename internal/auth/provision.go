package auth

import "strings"

// Access decides how an external identity is attached to a Nutzer.
// Members of the admin group are treated as allowed.
func Access(hasIdentity, emailMatch, emailVerified, allowEmailLink, inAdmin, inAllowed bool) string {
	switch {
	case hasIdentity:
		return "existing"
	case allowEmailLink && emailVerified && emailMatch:
		return "link"
	case inAdmin || inAllowed:
		return "create"
	default:
		return "reject"
	}
}

// InGroup reports whether name is in groups. Comparison is case-sensitive, as issued by the IdP.
func InGroup(groups []string, name string) bool {
	if name == "" {
		return false
	}
	for _, g := range groups {
		if g == name {
			return true
		}
	}
	return false
}

// ParseGroups accepts a JSON array or a comma-separated string.
func ParseGroups(raw []byte) ([]string, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return nil, false
	}
	if strings.HasPrefix(s, "[") {
		var items []string
		if err := unmarshalStringList(s, &items); err != nil {
			return nil, true
		}
		return compact(items), true
	}
	s = strings.Trim(s, `"`)
	return compact(strings.Split(s, ",")), true
}

func unmarshalStringList(s string, dst *[]string) error {
	// Local wrapper keeps encoding/json out of the hot import cycle for tests.
	return jsonUnmarshal([]byte(s), dst)
}

func compact(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
