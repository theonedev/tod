package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	// DefaultQueryCount is the default page size for list endpoints.
	DefaultQueryCount = 25
	// MaxQueryCount is the upper bound enforced by the OneDev server.
	MaxQueryCount = 100
)

// getEntityData fetches a single entity (issue, PR, build, etc.) from an
// `/~api/tod/<endpointSuffix>` endpoint, passing the reference and
// current project as query parameters.
func getEntityData(endpointSuffix, reference, currentProject string) ([]byte, error) {
	query := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}
	return apiGetBytes(endpointSuffix, query)
}

// queryEntities runs a paginated query against an
// `/~api/tod/<endpointSuffix>` endpoint (query-issues, query-pull-requests, etc.).
func queryEntities(endpointSuffix, project, currentProject, query string, offset, count int) ([]byte, error) {
	q := url.Values{
		"project":        {project},
		"currentProject": {currentProject},
		"query":          {query},
		"offset":         {fmt.Sprintf("%d", offset)},
		"count":          {fmt.Sprintf("%d", count)},
	}
	return apiGetBytes(endpointSuffix, q)
}

// postJSON sends a POST request with a JSON body to an
// `/~api/tod/<endpointSuffix>` endpoint and returns the raw response.
func postJSON(endpointSuffix string, query url.Values, payload interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}
	apiURL := config.ServerUrl + "/~api/tod/" + endpointSuffix
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return makeAPICall(req)
}

// postText sends a POST request with a text/plain body to an
// `/~api/tod/<endpointSuffix>` endpoint and returns the raw response.
func postText(endpointSuffix string, query url.Values, body string) ([]byte, error) {
	apiURL := config.ServerUrl + "/~api/tod/" + endpointSuffix
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	return makeAPICall(req)
}

// apiGetBytes performs an authenticated GET against a tod endpoint.
func apiGetBytes(endpointSuffix string, query url.Values) ([]byte, error) {
	apiURL := config.ServerUrl + "/~api/tod/" + endpointSuffix
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	return makeAPICall(req)
}

// apiGetAbsolute performs an authenticated GET against an absolute URL (not
// prefixed with the tod path). Used for raw file downloads and patches.
func apiGetAbsolute(absoluteURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", absoluteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	return makeAPICall(req)
}

func getBuildDetail(reference, currentProject string) (map[string]interface{}, error) {
	data, err := getEntityData("get-build", reference, currentProject)
	if err != nil {
		return nil, err
	}
	var build map[string]interface{}
	if err := json.Unmarshal(data, &build); err != nil {
		return nil, fmt.Errorf("failed to parse build response: %v", err)
	}
	return build, nil
}

func getPullRequestDetail(reference, currentProject string) (map[string]interface{}, error) {
	data, err := getEntityData("get-pull-request", reference, currentProject)
	if err != nil {
		return nil, err
	}
	var pr map[string]interface{}
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse pull request response: %v", err)
	}
	return pr, nil
}

func getPullRequestPatchInfo(reference, currentProject string) (map[string]interface{}, error) {
	data, err := apiGetBytes("get-pull-request-patch-info", url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	})
	if err != nil {
		return nil, err
	}
	var info map[string]interface{}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse patch info response: %v", err)
	}
	return info, nil
}

// parseFieldValue interprets a CLI `--field key=value` value. Numeric,
// boolean, null, array, and object JSON literals are preserved; everything
// else is treated as a raw string.
func parseFieldValue(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	if trimmed == "true" || trimmed == "false" || trimmed == "null" ||
		strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") ||
		strings.HasPrefix(trimmed, "\"") ||
		looksNumeric(trimmed) {
		var parsed interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed
		}
	}
	return raw
}

func looksNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r == '.' || r == 'e' || r == 'E' || r == '+' || r == '-' {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// collectFieldFlags parses repeated --field key=value flags into a map. If
// the same key is provided more than once the last wins (to keep behaviour
// predictable; use a single JSON array value for multi-value fields).
func collectFieldFlags(fields []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, entry := range fields {
		idx := strings.Index(entry, "=")
		if idx <= 0 {
			return nil, fmt.Errorf("invalid --field value %q: expected key=value", entry)
		}
		key := strings.TrimSpace(entry[:idx])
		if key == "" {
			return nil, fmt.Errorf("invalid --field value %q: key is empty", entry)
		}
		result[key] = parseFieldValue(entry[idx+1:])
	}
	return result, nil
}
