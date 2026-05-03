package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Inspect dynamic input schemas the OneDev server advertises (fields, required keys, enums)",
	Long: `Inspect dynamic input schemas the OneDev server advertises for server-driven
subcommands (for example valid fields of 'issue create', allowed states for
'issue transition', or valid link names for 'issue link').

Use these to discover server-specific values — including custom fields — that
cannot be hard-coded into the CLI. The raw JSON payload comes from the
/~api/tod/get-tool-input-schemas endpoint and mirrors what the
deprecated MCP server advertised in its tools/list response.`,
}

var schemaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the schema names this OneDev server exposes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, _, err := fetchSchemaNames()
		if err != nil {
			return err
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return nil
	},
}

var schemaShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print the JSON schema for one tool (accepts camelCase or kebab-case name)",
	Long: `Print the JSON schema for one tool as a pretty-printed JSON object with
'type', 'properties', and 'required' keys.

Name can be given in either camelCase (as the server returns it, e.g.
'createIssue') or kebab-case ('create-issue'). Pipe the output through jq to
extract just the parts you need, for example:

	tod schema show create-issue | jq -r '.properties | keys[]'
	tod schema show change-issue-state | jq '.properties.state'

Run 'tod schema list' to see the available names.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		names, schemas, err := fetchSchemaNames()
		if err != nil {
			return err
		}
		raw, ok := schemas[args[0]]
		if !ok {
			if camel := kebabToCamel(args[0]); camel != args[0] {
				raw, ok = schemas[camel]
			}
		}
		if !ok {
			return fmt.Errorf("unknown schema %q. Available: %s", args[0], strings.Join(names, ", "))
		}
		out, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format schema: %v", err)
		}
		emit(out)
		return nil
	},
}

// fetchSchemaNames returns the sorted list of schema names together with the
// full schemas map so callers can look up individual entries without issuing a
// second request.
func fetchSchemaNames() ([]string, map[string]interface{}, error) {
	schemas, err := getJSONMapFromAPI(config.ServerUrl + "/~api/tod/get-tool-input-schemas")
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, schemas, nil
}

// kebabToCamel converts e.g. "create-issue" to "createIssue" so CLI-idiomatic
// names resolve to the server's camelCase keys.
func kebabToCamel(s string) string {
	if !strings.Contains(s, "-") {
		return s
	}
	parts := strings.Split(s, "-")
	var b strings.Builder
	b.Grow(len(s))
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			b.WriteString(p)
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

func initSchemaCommands() {
	schemaCmd.AddCommand(schemaListCmd, schemaShowCmd)
}
