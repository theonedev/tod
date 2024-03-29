package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"io"

	"github.com/Masterminds/semver"
)

const version = "1.0.0"

type CompatibleVersions struct {
	MinVersion string `json:"minVersion"`
	MaxVersion string `json:"maxVersion"`
}

func main() {
	if len(os.Args) == 1 {
		fmt.Fprintln(os.Stderr, "Command expected. Check https://code.onedev.io/onedev/tod for details")
		os.Exit(1)
	}

	var command Command

	switch os.Args[1] {
	case "exec":
		command = ExecCommand{}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s. Check https://code.onedev.io/onedev/tod for details\n", os.Args[1])
		os.Exit(1)
	}

	command.Execute(os.Args[2:])
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			return fmt.Errorf("non-successful status code received (status: %s, message: %s)", resp.Status, string(body))
		} else {
			return fmt.Errorf("non-successful status code received: %s", resp.Status)
		}
	} else {
		return nil
	}
}

func checkVersion(serverUrl string, accessToken string) error {
	// Make a GET request to the API endpoint
	client := &http.Client{}

	// Create a new GET request
	req, err := http.NewRequest("GET", serverUrl+"/~api/version/compatible-tod-versions", nil)
	if err != nil {
		return fmt.Errorf("error requesting compatible versions: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Send the request using the HTTP client
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error requesting compatible versions: %w", err)
	}

	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return fmt.Errorf("error requesting compatible versions: %w", err)
	}

	// Decode the JSON response into a VersionInfo struct
	var compatibleVersions CompatibleVersions

	err = json.NewDecoder(resp.Body).Decode(&compatibleVersions)
	if err != nil {
		return fmt.Errorf("error decoding compatible versions response: %w", err)
	}

	semVer, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("error parsing semver: %w", err)
	}

	var minVersionSatisfied = true

	if compatibleVersions.MinVersion != "" {
		minSemVer, err := semver.NewVersion(compatibleVersions.MinVersion)
		if err != nil {
			return fmt.Errorf("error parsing semver: %w", err)
		}
		if semVer.LessThan(minSemVer) {
			minVersionSatisfied = false
		}
	}

	var maxVersionSatisfied = true

	if compatibleVersions.MaxVersion != "" {
		maxSemVer, err := semver.NewVersion(compatibleVersions.MaxVersion)
		if err != nil {
			return fmt.Errorf("error parsing semver: %w", err)
		}
		if semVer.GreaterThan(maxSemVer) {
			maxVersionSatisfied = false
		}
	}

	if minVersionSatisfied && maxVersionSatisfied {
		return nil
	} else if compatibleVersions.MinVersion != "" && compatibleVersions.MaxVersion != "" {
		return fmt.Errorf("this server requires version >= %s and version <= %s, please download from https://code.onedev.io/onedev/tod/~builds?query=%%22Job%%22+is+%%22Release%%22+and+successful", compatibleVersions.MinVersion, compatibleVersions.MaxVersion)
	} else if compatibleVersions.MinVersion != "" {
		return fmt.Errorf("this server requires version >= %s, please download from https://code.onedev.io/onedev/tod/~builds?query=%%22Job%%22+is+%%22Release%%22+and+successful", compatibleVersions.MinVersion)
	} else {
		return fmt.Errorf("this server requires version <= %s, please download from https://code.onedev.io/onedev/tod/~builds?query=%%22Job%%22+is+%%22Release%%22+and+successful", compatibleVersions.MaxVersion)
	}
}
