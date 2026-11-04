package main

import "strings"

type Scope struct {
	Orgs        []string
	Repos       []string
	ExcludeOrgs []string
}

func (s Scope) Query() string {
	var b strings.Builder
	add := func(qualifier string, values []string) {
		for _, v := range values {
			b.WriteString(qualifier)
			b.WriteString(v)
		}
	}
	add(" org:", s.Orgs)
	add(" repo:", s.Repos)
	add(" -org:", s.ExcludeOrgs)
	return b.String()
}

func (s Scope) Allows(fullName string) bool {
	owner, _, _ := strings.Cut(fullName, "/")
	if containsFold(s.ExcludeOrgs, owner) {
		return false
	}
	if len(s.Orgs) == 0 && len(s.Repos) == 0 {
		return true
	}
	return containsFold(s.Orgs, owner) || containsFold(s.Repos, fullName)
}

func parseList(csv string) []string {
	var out []string
	for v := range strings.SplitSeq(csv, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
