package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var checkVersionCmd = &cobra.Command{
	Use:    "check-version",
	Short:  "Check tod/server version compatibility",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("tod %s is compatible with OneDev server %s\n", version, checkedVersionInfo.ServerVersion)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:    "version",
	Short:  "Print the tod version",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(version)
		return nil
	},
}

var getLoginNameCmd = &cobra.Command{
	Use:   "get-login-name",
	Short: "Get the OneDev login name of the current user (or of --user)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		userName, _ := cmd.Flags().GetString("user")
		body, err := apiGetBytes("get-login-name", url.Values{"userName": {userName}})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var getUnixTimestampCmd = &cobra.Command{
	Use:   "get-unix-timestamp <datetime-description>",
	Short: "Convert a natural-language datetime description to a Unix timestamp (milliseconds)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-unix-timestamp", url.Values{"dateTimeDescription": {args[0]}})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Print the git remote that points at the inferred OneDev project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, _, err := inferProject(workingDirOf(cmd))
		if err != nil {
			return err
		}
		fmt.Println(remote)
		return nil
	},
}

var getValidLabelsCmd = &cobra.Command{
	Use:   "get-valid-labels",
	Short: "Print valid label names for this OneDev server",
	Long: `Print valid label names for this OneDev server. Use this to
discover which label names are accepted by --label when running
'tod pr create'.

The list is fetched from the OneDev server endpoint
/~api/tod/get-valid-labels.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-valid-labels", nil)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var getCommitMessageRequirementCmd = &cobra.Command{
	Use:   "get-commit-message-requirement [branch]",
	Short: "Print commit message requirement",
	Long: `Print commit message requirement for a branch.
The project is inferred from the current git repository's OneDev project.
Branch defaults to the current git branch when omitted.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		branch := ""
		if len(args) > 0 {
			branch = args[0]
		} else {
			branch, err = currentBranch(workingDirOf(cmd))
			if err != nil {
				return err
			}
			if branch == "" {
				return fmt.Errorf("branch is required: could not detect current branch (detached HEAD)")
			}
		}

		body, err := apiGetBytes("get-commit-message-requirement", url.Values{
			"project": {project},
			"branch":  {branch},
		})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var downloadCmd = &cobra.Command{
	Use:   "download <resource-url> <output-file>",
	Short: "Download a resource (image, file, etc.) referenced in markdown",
	Long: `Download a resource referenced in markdown and save it to a local file.

The resource URL must be the original URL from the markdown without modification.
Relative URLs are resolved against the configured server-url. Authentication uses
the configured access-token.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceURL := args[0]
		outputFile := args[1]

		downloadURL, err := resolveMarkdownResourceURL(config.ServerUrl, resourceURL)
		if err != nil {
			return err
		}

		body, err := apiGetAbsolute(downloadURL)
		if err != nil {
			return err
		}

		if err := os.WriteFile(outputFile, body, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %v", outputFile, err)
		}
		return nil
	},
}

// resolveMarkdownResourceURL returns an absolute URL for downloading a markdown
// resource. Absolute http(s) URLs are returned unchanged; relative URLs are
// resolved against serverURL.
func resolveMarkdownResourceURL(serverURL, resourceURL string) (string, error) {
	parsed, err := url.Parse(resourceURL)
	if err != nil {
		return "", fmt.Errorf("invalid resource URL %q: %v", resourceURL, err)
	}
	if parsed.IsAbs() && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return resourceURL, nil
	}

	base, err := url.Parse(strings.TrimRight(serverURL, "/"))
	if err != nil {
		return "", fmt.Errorf("invalid server URL %q: %v", serverURL, err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("invalid server URL %q: expected http or https scheme", serverURL)
	}

	resolved := base.ResolveReference(parsed)
	return resolved.String(), nil
}

func initMiscCommands() {
	getLoginNameCmd.Flags().String("user", "", "User name (defaults to the current user)")

	remoteCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")
	getCommitMessageRequirementCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")
}
