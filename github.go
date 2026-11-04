package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const apiBase = "https://api.github.com"

// PR is the part of a pull request that the rules look at.
type PR struct {
	URL            string
	Title          string
	Number         int
	Repo           string // owner/name
	Author         string
	IsDraft        bool
	UpdatedAt      time.Time
	ReviewDecision string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, or "" when reviews are optional
	Mergeable      string // MERGEABLE, CONFLICTING, UNKNOWN
	CI             string // SUCCESS, FAILURE, ERROR, PENDING, EXPECTED, or "" when there is no CI
	HeadOID        string
	RequestedUsers []string
	RequestedTeams []string
	Reviews        []Review // the latest review from each reviewer
}

// Review is the latest review of one reviewer.
type Review struct {
	Author      string
	State       string // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	CommitOID   string // the head commit when the reviewer sent the review
	SubmittedAt time.Time
}

// Mention is an unread notification that @mentions the user on a PR.
type Mention struct {
	URL       string
	Title     string
	Repo      string
	Number    int
	UpdatedAt time.Time
}

// Snapshot is everything one poll gets from GitHub.
type Snapshot struct {
	Me             string
	ReviewRequests []PR // review-requested:@me, which includes team requests
	Reviewed       []PR // PRs you reviewed earlier that do not request you now
	Mine           []PR // author:@me
	Mentions       []Mention
	RateRemaining  int
}

// Client talks to the GitHub API with one token.
type Client struct {
	token string
	http  *http.Client
}

// FindToken gets a token from GITHUB_TOKEN, GH_TOKEN, or `gh auth token`.
// It returns where the token came from, so the UI can show it.
func FindToken(ctx context.Context) (token, source string, err error) {
	for _, env := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if t := strings.TrimSpace(os.Getenv(env)); t != "" {
			return t, env, nil
		}
	}
	gh, err := findGH(ctx)
	if err != nil {
		return "", "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, gh, "auth", "token").Output()
	if err != nil {
		return "", "", fmt.Errorf("gh auth token: %w (run `gh auth login`, or set GITHUB_TOKEN)", err)
	}
	t := strings.TrimSpace(string(out))
	if t == "" {
		return "", "", errors.New("gh auth token returned nothing; run `gh auth login`")
	}
	return t, "gh", nil
}

// findGH finds the gh binary. An app that starts from Finder or launchd does not
// get the shell PATH. So findGH also checks the usual places, and at the end
// asks your login shell.
func findGH(ctx context.Context) (string, error) {
	if p, err := exec.LookPath("gh"); err == nil {
		return p, nil
	}
	places := []string{"/opt/homebrew/bin/gh", "/usr/local/bin/gh", "/home/linuxbrew/.linuxbrew/bin/gh"}
	if home, err := os.UserHomeDir(); err == nil {
		places = append(places, filepath.Join(home, "bin", "gh"), filepath.Join(home, ".local", "bin", "gh"))
	}
	for _, p := range places {
		if isExec(p) {
			return p, nil
		}
	}
	if p := ghFromLoginShell(ctx); p != "" {
		return p, nil
	}
	return "", errors.New("gh CLI not found; install it, or set GITHUB_TOKEN")
}

func ghFromLoginShell(ctx context.Context) string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, shell, "-lc", "command -v gh").Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) || !isExec(p) {
		return ""
	}
	return p
}

func isExec(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

func NewClient(token string) *Client {
	return &Client{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

const prFields = `
fragment pr on PullRequest {
  url title number isDraft updatedAt reviewDecision mergeable headRefOid
  repository { nameWithOwner }
  author { login }
  reviewRequests(first: 25) {
    nodes { requestedReviewer { __typename ... on User { login } ... on Team { slug } } }
  }
  latestReviews(first: 50) {
    nodes { author { login } state submittedAt commit { oid } }
  }
  commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
}`

// searchQuery runs one search per call. One call with three large searches
// often times out on GitHub's side with a 502.
const searchQuery = `
query($q: String!) {
  viewer { login }
  search(type: ISSUE, first: 50, query: $q) { nodes { ...pr } }
  rateLimit { remaining }
}` + prFields

type gqlPR struct {
	URL            string    `json:"url"`
	Title          string    `json:"title"`
	Number         int       `json:"number"`
	IsDraft        bool      `json:"isDraft"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ReviewDecision string    `json:"reviewDecision"`
	Mergeable      string    `json:"mergeable"`
	HeadRefOid     string    `json:"headRefOid"`
	Repository     struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer *struct {
				Typename string `json:"__typename"`
				Login    string `json:"login"`
				Slug     string `json:"slug"`
			} `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`
	LatestReviews struct {
		Nodes []struct {
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
			State       string    `json:"state"`
			SubmittedAt time.Time `json:"submittedAt"`
			Commit      *struct {
				Oid string `json:"oid"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"latestReviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

func (g gqlPR) toPR() PR {
	p := PR{
		URL:            g.URL,
		Title:          g.Title,
		Number:         g.Number,
		Repo:           g.Repository.NameWithOwner,
		IsDraft:        g.IsDraft,
		UpdatedAt:      g.UpdatedAt,
		ReviewDecision: g.ReviewDecision,
		Mergeable:      g.Mergeable,
		HeadOID:        g.HeadRefOid,
	}
	if g.Author != nil {
		p.Author = g.Author.Login
	}
	for _, n := range g.ReviewRequests.Nodes {
		switch r := n.RequestedReviewer; {
		case r == nil:
		case r.Typename == "User":
			p.RequestedUsers = append(p.RequestedUsers, r.Login)
		case r.Typename == "Team":
			p.RequestedTeams = append(p.RequestedTeams, r.Slug)
		}
	}
	for _, n := range g.LatestReviews.Nodes {
		r := Review{State: n.State, SubmittedAt: n.SubmittedAt}
		if n.Author != nil {
			r.Author = n.Author.Login
		}
		if n.Commit != nil {
			r.CommitOID = n.Commit.Oid
		}
		p.Reviews = append(p.Reviews, r)
	}
	if len(g.Commits.Nodes) > 0 && g.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
		p.CI = g.Commits.Nodes[0].Commit.StatusCheckRollup.State
	}
	return p
}

// Fetch gets one snapshot: one GraphQL call and one REST call.
func (c *Client) Fetch(ctx context.Context, excludeOrgs []string) (*Snapshot, error) {
	var b strings.Builder
	b.WriteString("is:open is:pr archived:false")
	for _, o := range excludeOrgs {
		b.WriteString(" -org:")
		b.WriteString(o)
	}
	base := b.String()

	s := &Snapshot{}
	for _, q := range []struct {
		query string
		out   *[]PR
	}{
		{base + " review-requested:@me", &s.ReviewRequests},
		{base + " reviewed-by:@me -author:@me -review-requested:@me", &s.Reviewed},
		{base + " author:@me sort:updated-desc", &s.Mine},
	} {
		var data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
			Search struct{ Nodes []gqlPR } `json:"search"`
			Rate   struct {
				Remaining int `json:"remaining"`
			} `json:"rateLimit"`
		}
		if err := c.graphql(ctx, searchQuery, map[string]any{"q": q.query}, &data); err != nil {
			return nil, err
		}
		s.Me, s.RateRemaining = data.Viewer.Login, data.Rate.Remaining
		for _, g := range data.Search.Nodes {
			if g.URL != "" { // search can return non-PR nodes as empty objects
				*q.out = append(*q.out, g.toPR())
			}
		}
	}

	// A failed mentions call must not hide the review list, so only log it.
	mentions, err := c.mentions(ctx, excludeOrgs)
	if err != nil {
		slog.Warn("fetch mentions", "err", err)
	}
	s.Mentions = mentions
	return s, nil
}

func (c *Client) graphql(ctx context.Context, query string, vars map[string]any, out any) error {
	body, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	raw, err := c.do(req)
	if err != nil {
		return err
	}
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("decode graphql: %w", err)
	}
	// Orgs with SAML SSO that the token cannot see give partial errors.
	// Keep the data from the other orgs.
	for _, e := range resp.Errors {
		slog.Warn("graphql", "type", e.Type, "msg", e.Message)
	}
	if len(resp.Data) == 0 || string(resp.Data) == "null" {
		if len(resp.Errors) > 0 {
			return fmt.Errorf("graphql: %s", resp.Errors[0].Message)
		}
		return errors.New("graphql: empty response")
	}
	return json.Unmarshal(resp.Data, out)
}

func (c *Client) mentions(ctx context.Context, excludeOrgs []string) ([]Mention, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/notifications?participating=true&per_page=50", nil)
	if err != nil {
		return nil, err
	}
	raw, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var threads []struct {
		Reason    string    `json:"reason"`
		Unread    bool      `json:"unread"`
		UpdatedAt time.Time `json:"updated_at"`
		Subject   struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Type  string `json:"type"`
		} `json:"subject"`
		Repository struct {
			FullName string `json:"full_name"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &threads); err != nil {
		return nil, fmt.Errorf("decode notifications: %w", err)
	}
	var out []Mention
	for _, t := range threads {
		if !t.Unread || t.Reason != "mention" || t.Subject.Type != "PullRequest" {
			continue
		}
		if containsFold(excludeOrgs, t.Repository.Owner.Login) {
			continue
		}
		u := strings.Replace(t.Subject.URL, apiBase+"/repos/", "https://github.com/", 1)
		u = strings.Replace(u, "/pulls/", "/pull/", 1)
		var n int
		fmt.Sscanf(u[strings.LastIndex(u, "/")+1:], "%d", &n)
		out = append(out, Mention{URL: u, Title: t.Subject.Title, Repo: t.Repository.FullName, Number: n, UpdatedAt: t.UpdatedAt})
	}
	return out, nil
}

// do sends the request. It tries again after a network error or a 5xx,
// because GitHub often gives a short 502 under load.
func (c *Client) do(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "honk")

	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	var lastErr error
	for attempt, wait := range []time.Duration{0, 2 * time.Second, 5 * time.Second} {
		if wait > 0 {
			slog.Info("retry", "attempt", attempt+1, "after", wait, "err", lastErr)
			select {
			case <-time.After(wait):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		raw, retry, err := c.once(req)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}
	return nil, lastErr
}

func (c *Client) once(req *http.Request) (raw []byte, retry bool, err error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()
	raw, err = io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, true, err
	}
	if resp.StatusCode == http.StatusOK {
		return raw, false, nil
	}
	err = fmt.Errorf("%s %s: %s", req.Method, req.URL.Path, resp.Status)
	if msg := bytes.TrimSpace(raw[:min(len(raw), 200)]); len(msg) > 0 && msg[0] != '<' { // skip HTML error pages
		err = fmt.Errorf("%w: %s", err, msg)
	}
	return nil, resp.StatusCode >= 500, err
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
