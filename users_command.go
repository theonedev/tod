package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type UsersCommand struct {
}

func (command UsersCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	count, _ := cobraCmd.Flags().GetInt("count")

	apiURL := config.ServerUrl + fmt.Sprintf("/~api/users?offset=0&count=%d", count)

	body, err := makeAPICallSimple("GET", apiURL, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to query users:", err)
		os.Exit(1)
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(body, &users); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse users:", err)
		os.Exit(1)
	}

	if len(users) == 0 {
		fmt.Println("No users found.")
		return
	}

	for _, user := range users {
		id := int(user["id"].(float64))
		name, _ := user["name"].(string)
		fullName, _ := user["fullName"].(string)
		email, _ := user["emailAddress"].(string)

		if fullName != "" {
			fmt.Printf("%-4d %-20s %-25s %s\n", id, name, fullName, email)
		} else {
			fmt.Printf("%-4d %-20s %s\n", id, name, email)
		}
	}
}
