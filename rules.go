package main

import (
	"sort"
	"strings"
	"time"
)

// Kind says what you must do on a PR.
type Kind string

const (
	// Someone waits on you. These honk.
	KindReview     Kind = "review"
	KindReReview   Kind = "re-review"
	KindMention    Kind = "mention"
	KindTeamReview Kind = "team review" // honks only with -team-honk

	// Your own PR waits on you. These chirp.
	KindFixConflict Kind = "fix conflict"
	KindFixTests    Kind = "fix tests"
	KindRespond     Kind = "respond"
	KindMerge       Kind = "merge"
)

var kindIcon = map[Kind]string{
	KindReview:      "👀",
	KindReReview:    "🔁",
	KindMention:     "💬",
	KindTeamReview:  "👥",
	KindFixConflict: "⚔️",
	KindFixTests:    "🪳",
	KindRespond:     "🪚",
	KindMerge:       "🚀",
}

// Action is one thing you must do on one PR.
type Action struct {
	Kind      Kind
	URL       string
	Repo      string
	Number    int
	Title     string
	Reason    string
	UpdatedAt time.Time
	Incoming  bool // true when someone else waits on you
}

// Key is the identity of an action. A change of kind on the same PR is a new action.
func (a Action) Key() string { return a.URL + "#" + string(a.Kind) }

type Options struct {
	StaleAfter time.Duration // skip PRs with no update for this long; 0 keeps all
	TeamHonk   bool
}

// Decide turns a snapshot into a list of actions. It is a pure function, so the
// rules are easy to test and to change.
func Decide(s *Snapshot, now time.Time, opt Options) []Action {
	var out []Action
	seen := map[string]bool{}
	add := func(a Action) {
		if seen[a.URL] {
			return // one action per PR; the first rule that matches wins
		}
		seen[a.URL] = true
		out = append(out, a)
	}
	stale := func(t time.Time) bool { return opt.StaleAfter > 0 && now.Sub(t) > opt.StaleAfter }

	// 1. Someone asked you for a review.
	for _, p := range s.ReviewRequests {
		if p.IsDraft || stale(p.UpdatedAt) || strings.EqualFold(p.Author, s.Me) {
			continue
		}
		if containsFold(p.RequestedUsers, s.Me) {
			add(action(p, KindReview, true, "@"+p.Author+" asked you to review"))
		} else {
			add(action(p, KindTeamReview, true, "review requested from "+teamList(p.RequestedTeams)))
		}
	}

	// 2. You asked for changes, and the author pushed new commits since then.
	// Your review still blocks the merge, so it is your turn again.
	for _, p := range s.Reviewed {
		if p.IsDraft || stale(p.UpdatedAt) {
			continue
		}
		r, ok := latestReviewBy(p, s.Me)
		if ok && r.State == "CHANGES_REQUESTED" && r.CommitOID != "" && r.CommitOID != p.HeadOID {
			add(action(p, KindReReview, true, "@"+p.Author+" pushed after you asked for changes"))
		}
	}

	// 3. Someone @mentioned you and you did not open the PR since.
	for _, m := range s.Mentions {
		if stale(m.UpdatedAt) {
			continue
		}
		add(Action{Kind: KindMention, URL: m.URL, Repo: m.Repo, Number: m.Number, Title: m.Title,
			Reason: "you were @mentioned", UpdatedAt: m.UpdatedAt, Incoming: true})
	}

	// 4. Your own PRs. Order matters: the first match is the most urgent.
	for _, p := range s.Mine {
		if p.IsDraft || stale(p.UpdatedAt) {
			continue
		}
		switch {
		case p.Mergeable == "CONFLICTING":
			add(action(p, KindFixConflict, false, "merge conflict with the base branch"))
		case p.CI == "FAILURE" || p.CI == "ERROR":
			add(action(p, KindFixTests, false, "CI failed"))
		case changesRequestedOnHead(p):
			add(action(p, KindRespond, false, "changes requested"))
		case approved(p) && (p.CI == "SUCCESS" || p.CI == ""):
			add(action(p, KindMerge, false, "approved and green"))
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Incoming != out[j].Incoming {
			return out[i].Incoming
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func action(p PR, k Kind, incoming bool, reason string) Action {
	return Action{Kind: k, URL: p.URL, Repo: p.Repo, Number: p.Number, Title: p.Title,
		Reason: reason, UpdatedAt: p.UpdatedAt, Incoming: incoming}
}

func latestReviewBy(p PR, login string) (Review, bool) {
	for _, r := range p.Reviews {
		if strings.EqualFold(r.Author, login) {
			return r, true
		}
	}
	return Review{}, false
}

// changesRequestedOnHead is true when a reviewer asked for changes on the
// current head commit. After you push, the turn goes back to the reviewer.
func changesRequestedOnHead(p PR) bool {
	if p.ReviewDecision != "" && p.ReviewDecision != "CHANGES_REQUESTED" {
		return false
	}
	for _, r := range p.Reviews {
		if r.State == "CHANGES_REQUESTED" && (r.CommitOID == "" || r.CommitOID == p.HeadOID) {
			return true
		}
	}
	return false
}

// approved is true when branch protection says so, or, in repos where reviews
// are optional, when one reviewer approved and no reviewer asked for changes.
func approved(p PR) bool {
	if p.ReviewDecision != "" {
		return p.ReviewDecision == "APPROVED"
	}
	ok := false
	for _, r := range p.Reviews {
		switch r.State {
		case "APPROVED":
			ok = true
		case "CHANGES_REQUESTED":
			return false
		}
	}
	return ok
}

func teamList(teams []string) string {
	if len(teams) == 0 {
		return "your team"
	}
	return "@" + strings.Join(teams, ", @")
}

// Honks says if a new action of this kind honks (true) or chirps (false).
func (a Action) Honks(opt Options) bool {
	if a.Kind == KindTeamReview {
		return opt.TeamHonk
	}
	return a.Incoming
}
