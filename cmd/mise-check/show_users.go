package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// user is one of the owner's repos that uses this one: as a Go module (its go.mod requires it), as a mise tool (a mise
// file pins its binary, github:<owner>/<repo>), or both, at the versions it pins.
type user struct {
	Repo   string `json:"repo"`             // owner/name
	Go     string `json:"go,omitempty"`     // the version its go.mod requires
	Mise   string `json:"mise,omitempty"`   // the version a mise file pins, and which file: "0.4.2 (.mise/config.toml)"
	Behind bool   `json:"behind,omitempty"` // a pin older than the newest release
}

// showUsers is mise-check show-users: which of the owner's repos use this repo, by its Go module (go.mod) and by its
// binary as a mise tool (.mise/config.toml, mise.toml), at the versions each pins, against the newest release. The
// owner (a user or an organization) and the name are the module path's, github.com/<owner>/<name>. It reads every repo
// of the owner on GitHub, not forks or archived ones, with a token when there is one (GITHUB_TOKEN, GH_TOKEN, or gh's
// login), which private repos need.
func (r repo) showUsers(jsonOut bool) {
	mod := r.modulePath()
	parts := strings.Split(mod, "/")
	if len(parts) < 3 || parts[0] != "github.com" {
		fail(fmt.Errorf("the module %s isn't on GitHub (github.com/<owner>/<name>): show-users reads the owner's repos there", mod))
	}
	owner, name := parts[1], parts[2]
	gh := github{token: githubToken()}
	newest := strings.TrimPrefix(gh.newestRelease(owner, name), "v")
	repos := gh.repos(owner)

	goReq := regexp.MustCompile(`(?m)^\s*(?:require\s+)?` + regexp.QuoteMeta(mod) + `\s+v(\S+)`)
	misePin := regexp.MustCompile(`"github:` + regexp.QuoteMeta(owner+"/"+name) + `"\s*=\s*(?:"([^"]*)"|\{[^}]*version\s*=\s*"([^"]*)")`)
	var mu sync.Mutex
	var users []user
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, full := range repos {
		if full == owner+"/"+name {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			u := user{Repo: full}
			if m := goReq.FindStringSubmatch(gh.file(full, "go.mod")); m != nil {
				u.Go = m[1]
			}
			for _, f := range []string{".mise/config.toml", "mise.toml"} {
				if m := misePin.FindStringSubmatch(gh.file(full, f)); m != nil {
					u.Mise = cmp.Or(m[1], m[2]) + " (" + f + ")"
					break
				}
			}
			if u.Go == "" && u.Mise == "" {
				return
			}
			pin := strings.Fields(u.Mise + " ")[0]
			u.Behind = newest != "" && ((u.Go != "" && older(u.Go, newest)) || (pin != "" && pin != "latest" && pin != "local" && older(pin, newest)))
			mu.Lock()
			users = append(users, u)
			mu.Unlock()
		}()
	}
	wg.Wait()
	slices.SortFunc(users, func(a, b user) int { return strings.Compare(a.Repo, b.Repo) })

	if jsonOut {
		emit(struct {
			Module string `json:"module"`
			Newest string `json:"newest"`
			Users  []user `json:"users"`
		}{mod, newest, append([]user{}, users...)})
		return
	}
	fmt.Printf("%s: newest release %s; %d of %s's %d repos use it\n", mod, cmp.Or(newest, "none"), len(users), owner, len(repos))
	for _, u := range users {
		var how []string
		if u.Go != "" {
			how = append(how, "go.mod "+u.Go)
		}
		if u.Mise != "" {
			how = append(how, "mise "+u.Mise)
		}
		mark := "✓"
		if u.Behind {
			mark = "↑"
		}
		fmt.Printf("%s %s: %s\n", mark, u.Repo, strings.Join(how, ", "))
	}
	if slices.ContainsFunc(users, func(u user) bool { return u.Behind }) {
		fmt.Printf("↑ behind %s: in each, mise run mise-check:update moves it (update-go for go.mod, update-mise for mise)\n", newest)
	}
}

// modulePath is the module path of the repo's go.mod.
func (r repo) modulePath() string {
	var f struct{ Module struct{ Path string } }
	out := r.miseIn(r.dir, "exec", "--", "go", "mod", "edit", "-json")
	if out == "" {
		if b, err := exec.Command("go", "mod", "edit", "-json").Output(); err == nil {
			out = string(b)
		}
	}
	if json.Unmarshal([]byte(out), &f) != nil || f.Module.Path == "" {
		fail(errors.New("no go.mod here: show-users finds the repos that use this one by its module path"))
	}
	return f.Module.Path
}

// githubToken is a token for GitHub's API: GITHUB_TOKEN, GH_TOKEN, or gh's own login; "" without any, for public repos
// only (and GitHub's lower rate limit).
func githubToken() string {
	if t := cmp.Or(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN")); t != "" {
		return t
	}
	if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// github is GitHub's REST API, with a token when there is one.
type github struct{ token string }

var client = &http.Client{Timeout: 30 * time.Second}

// get is the body of GitHub's answer to the API path, "" and the status when it isn't 200.
func (g github) get(path, accept string) (string, int) {
	req, err := http.NewRequest("GET", "https://api.github.com/"+path, nil)
	must(err)
	req.Header.Set("Accept", cmp.Or(accept, "application/vnd.github+json"))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := client.Do(req)
	if err != nil {
		fail(fmt.Errorf("GitHub's API: %w", err))
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode
	}
	return string(b), resp.StatusCode
}

// repos are the owner's repos, owner/name, not forks or archived ones: an organization's, or a user's.
func (g github) repos(owner string) []string {
	var all []string
	for _, kind := range []string{"orgs", "users"} {
		for page := 1; ; page++ {
			body, status := g.get(fmt.Sprintf("%s/%s/repos?per_page=100&page=%d", kind, owner, page), "")
			if status == http.StatusNotFound && kind == "orgs" && page == 1 {
				break // not an organization: a user
			}
			if status != http.StatusOK {
				fail(fmt.Errorf("GitHub refused the list of %s's repos (HTTP %d): a token that can read them is GITHUB_TOKEN, GH_TOKEN or gh auth login", owner, status))
			}
			var repos []struct {
				FullName string `json:"full_name"`
				Fork     bool
				Archived bool
			}
			must(json.Unmarshal([]byte(body), &repos))
			for _, r := range repos {
				if !r.Fork && !r.Archived {
					all = append(all, r.FullName)
				}
			}
			if len(repos) < 100 {
				return all
			}
		}
	}
	return all
}

// file is a file of the repo's default branch, "" when it has none.
func (g github) file(repo, path string) string {
	body, _ := g.get("repos/"+repo+"/contents/"+path, "application/vnd.github.raw")
	return body
}

// newestRelease is the repo's newest release's tag, "" when it has none.
func (g github) newestRelease(owner, name string) string {
	body, _ := g.get("repos/"+owner+"/"+name+"/releases/latest", "")
	var rel struct {
		TagName string `json:"tag_name"`
	}
	json.Unmarshal([]byte(body), &rel)
	return rel.TagName
}
