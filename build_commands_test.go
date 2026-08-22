package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBuildUnitTestReport(t *testing.T) {
	tests := []struct {
		name           string
		artifact       string
		expectArtifact string
		expectResponse string
	}{
		{
			name:           "report data",
			expectResponse: `{"testSuites":[]}`,
		},
		{
			name:           "artifact",
			artifact:       "artifacts/junit/failure.txt",
			expectArtifact: "artifacts/junit/failure.txt",
			expectResponse: "failure details",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/~api/tod/get-build-unit-test-report" {
					t.Fatalf("unexpected request path: %s", r.URL.Path)
				}
				query := r.URL.Query()
				if query.Get("currentProject") != "acme/project" {
					t.Fatalf("unexpected currentProject: %q", query.Get("currentProject"))
				}
				if query.Get("reference") != "#42" {
					t.Fatalf("unexpected reference: %q", query.Get("reference"))
				}
				if query.Get("reportName") != "junit" {
					t.Fatalf("unexpected reportName: %q", query.Get("reportName"))
				}
				if query.Get("artifactPath") != test.expectArtifact {
					t.Fatalf("unexpected artifactPath: %q", query.Get("artifactPath"))
				}
				fmt.Fprint(w, test.expectResponse)
			}))
			defer server.Close()

			oldConfig := config
			config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
			defer func() { config = oldConfig }()

			body, err := getBuildUnitTestReport("#42", "acme/project", "junit", test.artifact)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != test.expectResponse {
				t.Fatalf("unexpected response: %q", string(body))
			}
		})
	}
}
