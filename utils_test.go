package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckVersionReportsNonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/~api/tod/check-version" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html><body>Proxy error</body></html>")
	}))
	defer server.Close()

	_, err := checkVersion(&Config{
		ServerUrl:   server.URL,
		AccessToken: "test-token",
	})
	if err == nil {
		t.Fatal("expected checkVersion to reject an HTML response")
	}

	message := err.Error()
	for _, expected := range []string{
		server.URL + "/~api/tod/check-version",
		`content-type: "text/html; charset=utf-8"`,
		`response: "<html><body>Proxy error</body></html>"`,
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("expected error to contain %q, got %q", expected, message)
		}
	}
}

func TestRemoteURLForProjectPreservesHTTPRemoteShape(t *testing.T) {
	got, err := remoteURLForProject("https://example.com/old/project.git", "new/project")
	if err != nil {
		t.Fatal(err)
	}

	want := "https://example.com/new/project.git"
	if got != want {
		t.Fatalf("unexpected remote URL: got %q, want %q", got, want)
	}
}

func TestRemoteURLForProjectPreservesSSHRemoteShape(t *testing.T) {
	got, err := remoteURLForProject("ssh://git@example.com/old/project", "new/project")
	if err != nil {
		t.Fatal(err)
	}

	want := "ssh://git@example.com/new/project"
	if got != want {
		t.Fatalf("unexpected remote URL: got %q, want %q", got, want)
	}
}
