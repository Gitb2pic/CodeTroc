package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// fakeGitHub simule l'API GitHub lue par la GitHub App.
type fakeGitHub struct {
	mu     sync.Mutex
	repos  map[string]Repository  // "owner/name" en minuscules
	pulls  map[string]PullRequest // "owner/name#n"
	checks map[string][]CheckRun  // sha
	err    error                  // si non nil, toute requête échoue
}

func newFakeGitHub() *fakeGitHub {
	return &fakeGitHub{repos: map[string]Repository{}, pulls: map[string]PullRequest{}, checks: map[string][]CheckRun{}}
}

func repoKey(owner, name string) string { return strings.ToLower(owner + "/" + name) }

func (f *fakeGitHub) Repository(_ context.Context, owner, name string) (Repository, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Repository{}, f.err
	}
	r, ok := f.repos[repoKey(owner, name)]
	if !ok {
		return Repository{}, ErrGitHubNotFound
	}
	return r, nil
}

func (f *fakeGitHub) PullRequest(_ context.Context, owner, name string, n int) (PullRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return PullRequest{}, f.err
	}
	p, ok := f.pulls[fmt.Sprintf("%s#%d", repoKey(owner, name), n)]
	if !ok {
		return PullRequest{}, ErrGitHubNotFound
	}
	return p, nil
}

func (f *fakeGitHub) CheckRuns(_ context.Context, _, _, sha string) ([]CheckRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.checks[sha], nil
}

func (f *fakeGitHub) addRepo(owner, name string, id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[repoKey(owner, name)] = Repository{ID: id, FullName: owner + "/" + name}
}

func (f *fakeGitHub) setPull(owner, name string, p PullRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pulls[fmt.Sprintf("%s#%d", repoKey(owner, name), p.Number)] = p
}

func (f *fakeGitHub) setChecks(sha string, runs ...CheckRun) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks[sha] = runs
}
