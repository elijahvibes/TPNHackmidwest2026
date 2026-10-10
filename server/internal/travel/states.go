package travel

import "strings"

// StateName returns the title-cased full name ("New York") for a state code
// or name; "" when unknown. StateCode and stateNames live in gazetteer.go.
func StateName(s string) string {
	name := stateNames[strings.ToUpper(StateCode(s))]
	if name == "" {
		return ""
	}
	words := strings.Fields(name)
	for i, w := range words {
		if w == "of" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
