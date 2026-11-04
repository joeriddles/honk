package main

import (
	"testing"
	"time"
)

var now = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func pr(url string, mod func(*PR)) PR {
	p := PR{URL: url, Repo: "o/r", Author: "alice", UpdatedAt: now.Add(-time.Hour), HeadOID: "head", Mergeable: "MERGEABLE"}
	if mod != nil {
		mod(&p)
	}
	return p
}

func kinds(as []Action) map[string]Kind {
	m := map[string]Kind{}
	for _, a := range as {
		m[a.URL] = a.Kind
	}
	return m
}

func TestDecide(t *testing.T) {
	s := &Snapshot{
		Me: "joe",
		ReviewRequests: []PR{
			pr("direct", func(p *PR) { p.RequestedUsers = []string{"Joe"} }),
			pr("team", func(p *PR) { p.RequestedTeams = []string{"nex"} }),
			pr("draft", func(p *PR) { p.IsDraft = true; p.RequestedUsers = []string{"joe"} }),
			pr("stale", func(p *PR) { p.UpdatedAt = now.Add(-100 * 24 * time.Hour); p.RequestedUsers = []string{"joe"} }),
		},
		Reviewed: []PR{
			pr("pushed-after-changes", func(p *PR) {
				p.Reviews = []Review{{Author: "joe", State: "CHANGES_REQUESTED", CommitOID: "old"}}
			}),
			pr("no-push-yet", func(p *PR) {
				p.Reviews = []Review{{Author: "joe", State: "CHANGES_REQUESTED", CommitOID: "head"}}
			}),
			pr("only-commented", func(p *PR) {
				p.Reviews = []Review{{Author: "joe", State: "COMMENTED", CommitOID: "old"}}
			}),
		},
		Mentions: []Mention{
			{URL: "mention", UpdatedAt: now},
			{URL: "direct", UpdatedAt: now}, // already a review; one action per PR
		},
		Mine: []PR{
			pr("conflict", func(p *PR) { p.Mergeable = "CONFLICTING"; p.CI = "FAILURE" }),
			pr("red", func(p *PR) { p.CI = "FAILURE" }),
			pr("changes", func(p *PR) {
				p.ReviewDecision = "CHANGES_REQUESTED"
				p.CI = "SUCCESS"
				p.Reviews = []Review{{Author: "bob", State: "CHANGES_REQUESTED", CommitOID: "head"}}
			}),
			pr("changes-then-pushed", func(p *PR) {
				p.ReviewDecision = "CHANGES_REQUESTED"
				p.Reviews = []Review{{Author: "bob", State: "CHANGES_REQUESTED", CommitOID: "old"}}
			}),
			pr("approved", func(p *PR) { p.ReviewDecision = "APPROVED"; p.CI = "SUCCESS" }),
			pr("approved-pending-ci", func(p *PR) { p.ReviewDecision = "APPROVED"; p.CI = "PENDING" }),
			pr("optional-review-approved", func(p *PR) { p.Reviews = []Review{{Author: "bob", State: "APPROVED"}} }),
			pr("waiting", func(p *PR) { p.ReviewDecision = "REVIEW_REQUIRED"; p.CI = "SUCCESS" }),
		},
	}

	got := kinds(Decide(s, now, Options{StaleAfter: 60 * 24 * time.Hour}))
	want := map[string]Kind{
		"direct":                   KindReview,
		"team":                     KindTeamReview,
		"pushed-after-changes":     KindReReview,
		"mention":                  KindMention,
		"conflict":                 KindFixConflict,
		"red":                      KindFixTests,
		"changes":                  KindRespond,
		"approved":                 KindMerge,
		"optional-review-approved": KindMerge,
	}
	for url, k := range want {
		if got[url] != k {
			t.Errorf("%s: got %q, want %q", url, got[url], k)
		}
	}
	for url, k := range got {
		if _, ok := want[url]; !ok {
			t.Errorf("%s: unexpected action %q", url, k)
		}
	}
}

func TestIncomingSortsFirst(t *testing.T) {
	s := &Snapshot{
		Me:             "joe",
		Mine:           []PR{pr("mine", func(p *PR) { p.CI = "FAILURE"; p.UpdatedAt = now })},
		ReviewRequests: []PR{pr("theirs", func(p *PR) { p.RequestedUsers = []string{"joe"}; p.UpdatedAt = now.Add(-48 * time.Hour) })},
	}
	as := Decide(s, now, Options{})
	if len(as) != 2 || as[0].URL != "theirs" {
		t.Fatalf("got %+v, want the incoming PR first", as)
	}
}

func TestHonks(t *testing.T) {
	if !(Action{Kind: KindReview, Incoming: true}).Honks(Options{}) {
		t.Error("a direct review must honk")
	}
	if (Action{Kind: KindTeamReview, Incoming: true}).Honks(Options{}) {
		t.Error("a team review must not honk by default")
	}
	if (Action{Kind: KindMerge}).Honks(Options{}) {
		t.Error("your own PR must chirp, not honk")
	}
}
