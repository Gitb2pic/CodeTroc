package app

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

// fakeIssue est une issue du faux GitHub.
type fakeIssue struct {
	Title, Body, State, Assignee string
	Labels                       []string
	Milestone                    int
	Comments                     []string
}

// fakeIssues simule l'écriture GitHub (labels, milestones, issues) d'un dépôt.
type fakeIssues struct {
	mu         sync.Mutex
	labels     map[string]bool
	milestones map[string]int     // titre → numéro
	due        map[int]time.Time  // numéro → échéance
	issues     map[int]*fakeIssue // numéro → issue (milestones et issues partagent la numérotation)
	repos      []int64            // dépôts visés, dans l'ordre
	next       int
	fail       func(op string) error // erreur injectée, nil sinon
	calls      int
}

func newFakeIssues() *fakeIssues {
	return &fakeIssues{labels: map[string]bool{}, milestones: map[string]int{}, due: map[int]time.Time{}, issues: map[int]*fakeIssue{}}
}

func (f *fakeIssues) begin(op string, repo RepoRef) error {
	f.calls++
	f.repos = append(f.repos, repo.ID)
	if f.fail != nil {
		return f.fail(op)
	}
	return nil
}

func (f *fakeIssues) EnsureLabels(_ context.Context, repo RepoRef, ls []scrum.Label) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.begin("labels", repo); err != nil {
		return err
	}
	for _, l := range ls {
		f.labels[l.Name] = true
	}
	return nil
}

func (f *fakeIssues) EnsureMilestone(_ context.Context, repo RepoRef, title string, due time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.begin("milestone", repo); err != nil {
		return 0, err
	}
	n, ok := f.milestones[title]
	if !ok {
		f.next++
		n = f.next
		f.milestones[title] = n
	}
	f.due[n] = due
	return n, nil
}

func (f *fakeIssues) CreateIssue(_ context.Context, repo RepoRef, in IssueInput) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.begin("create", repo); err != nil {
		return 0, err
	}
	f.next++
	f.issues[f.next] = &fakeIssue{Title: in.Title, Body: in.Body, State: "open", Assignee: in.Assignee, Labels: in.Labels, Milestone: in.Milestone}
	return f.next, nil
}

func (f *fakeIssues) UpdateIssue(_ context.Context, repo RepoRef, n int, in IssueUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.begin("update", repo); err != nil {
		return err
	}
	is, ok := f.issues[n]
	if !ok {
		return ErrGitHubNotFound
	}
	if in.Title != "" {
		is.Title = in.Title
	}
	if in.Body != "" {
		is.Body = in.Body
	}
	if in.State != "" {
		is.State = in.State
	}
	if in.Labels != nil {
		is.Labels = in.Labels
	}
	return nil
}

func (f *fakeIssues) Comment(_ context.Context, repo RepoRef, n int, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.begin("comment", repo); err != nil {
		return err
	}
	is, ok := f.issues[n]
	if !ok {
		return ErrGitHubNotFound
	}
	is.Comments = append(is.Comments, body)
	return nil
}

func (f *fakeIssues) issue(n int) fakeIssue {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.issues[n]
}

func (f *fakeIssues) delete(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.issues, n)
}

func (f *fakeIssues) setFail(fn func(op string) error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = fn
}

func (f *fakeIssues) hasLabel(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.labels[name]
}

func (f *fakeIssues) onlyRepo(id int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !slices.ContainsFunc(f.repos, func(r int64) bool { return r != id })
}
