package main

import "testing"

func TestScopeQuery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope Scope
		want  string
	}{
		{"everything", Scope{}, ""},
		{"exclude only", Scope{ExcludeOrgs: []string{"a", "b"}}, " -org:a -org:b"},
		{"orgs", Scope{Orgs: []string{"work"}}, " org:work"},
		{"repos", Scope{Repos: []string{"me/tool"}}, " repo:me/tool"},
		{"all three", Scope{Orgs: []string{"work"}, Repos: []string{"me/tool"}, ExcludeOrgs: []string{"old"}}, " org:work repo:me/tool -org:old"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.Query(); got != tc.want {
				t.Errorf("Query() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestScopeAllows(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope Scope
		repo  string
		want  bool
	}{
		{"empty scope allows all", Scope{}, "any/thing", true},
		{"excluded org", Scope{ExcludeOrgs: []string{"Old"}}, "old/x", false},
		{"org allowlist match ignores case", Scope{Orgs: []string{"Work"}}, "work/api", true},
		{"org allowlist miss", Scope{Orgs: []string{"work"}}, "personal/api", false},
		{"repo allowlist match", Scope{Repos: []string{"me/tool"}}, "Me/Tool", true},
		{"repo allowlist miss", Scope{Repos: []string{"me/tool"}}, "me/other", false},
		{"org or repo matches", Scope{Orgs: []string{"work"}, Repos: []string{"me/tool"}}, "me/tool", true},
		{"exclude beats allow", Scope{Orgs: []string{"work"}, ExcludeOrgs: []string{"work"}}, "work/api", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.Allows(tc.repo); got != tc.want {
				t.Errorf("Allows(%q) = %v, want %v", tc.repo, got, tc.want)
			}
		})
	}
}

func TestParseList(t *testing.T) {
	got := parseList(" a, ,b ,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("parseList = %q, want [a b]", got)
	}
}
