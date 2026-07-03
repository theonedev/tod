package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetIssueDetailForCheckoutSendsForWrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/~api/tod/get-issue" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("currentProject") != "acme/project" {
			t.Fatalf("unexpected currentProject: %q", query.Get("currentProject"))
		}
		if query.Get("reference") != "#42" {
			t.Fatalf("unexpected reference: %q", query.Get("reference"))
		}
		if query.Get("forWrite") != "true" {
			t.Fatalf("unexpected forWrite: %q", query.Get("forWrite"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"Project":"acme/project"}`)
	}))
	defer server.Close()

	oldConfig := config
	config = &Config{
		ServerUrl:   server.URL,
		AccessToken: "test-token",
	}
	defer func() {
		config = oldConfig
	}()

	issue, err := getIssueDetailForCheckout("#42", "acme/project", true)
	if err != nil {
		t.Fatal(err)
	}
	if issue["Project"] != "acme/project" {
		t.Fatalf("unexpected issue project: %#v", issue["Project"])
	}
}

func TestGetPullRequestDetailForCheckoutSendsForWriteWhenRequested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/~api/tod/get-pull-request" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("currentProject") != "acme/project" {
			t.Fatalf("unexpected currentProject: %q", query.Get("currentProject"))
		}
		if query.Get("reference") != "#43" {
			t.Fatalf("unexpected reference: %q", query.Get("reference"))
		}
		if query.Get("forWrite") != "true" {
			t.Fatalf("unexpected forWrite: %q", query.Get("forWrite"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"targetProject":"acme/project"}`)
	}))
	defer server.Close()

	oldConfig := config
	config = &Config{
		ServerUrl:   server.URL,
		AccessToken: "test-token",
	}
	defer func() {
		config = oldConfig
	}()

	pr, err := getPullRequestDetailForCheckout("#43", "acme/project", true)
	if err != nil {
		t.Fatal(err)
	}
	if pr["targetProject"] != "acme/project" {
		t.Fatalf("unexpected pull request target project: %#v", pr["targetProject"])
	}
}

func TestGetPullRequestDetailForCheckoutOmitsForWriteByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("forWrite"); got != "" {
			t.Fatalf("unexpected forWrite: %q", got)
		}
		fmt.Fprint(w, `{"targetProject":"acme/project"}`)
	}))
	defer server.Close()

	oldConfig := config
	config = &Config{
		ServerUrl:   server.URL,
		AccessToken: "test-token",
	}
	defer func() {
		config = oldConfig
	}()

	if _, err := getPullRequestDetailForCheckout("#43", "acme/project", false); err != nil {
		t.Fatal(err)
	}
}
