package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	apiURL string
	output string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gothinkdb-cli",
		Short: "GoThinkDB command-line interface",
		Long:  "A command-line interface for managing GoThinkDB",
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "API server URL")
	rootCmd.PersistentFlags().StringVar(&output, "output", "table", "Output format (table, json)")

	// Add commands
	rootCmd.AddCommand(serverCmd())
	rootCmd.AddCommand(databaseCmd())
	rootCmd.AddCommand(tableCmd())
	rootCmd.AddCommand(clusterCmd())
	rootCmd.AddCommand(queryCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Server management commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show server information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doGet("/api/server/info")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Show server statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doGet("/api/server/stats")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "health",
		Short: "Check server health",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doGet("/api/health")
		},
	})

	return cmd
}

func databaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doGet("/api/databases")
		},
	}

	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doPost("/api/databases", map[string]string{"name": args[0]})
		},
	}

	dropCmd := &cobra.Command{
		Use:   "drop [name]",
		Short: "Drop a database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doDelete("/api/databases/" + args[0])
		},
	}

	cmd.AddCommand(listCmd, createCmd, dropCmd)
	return cmd
}

func tableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "table",
		Short: "Table management commands",
	}

	var dbName string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doGet("/api/tables?db=" + dbName)
		},
	}
	listCmd.Flags().StringVar(&dbName, "db", "test", "Database name")

	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a table",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doPost("/api/tables/"+args[0]+"?db="+dbName, nil)
		},
	}
	createCmd.Flags().StringVar(&dbName, "db", "test", "Database name")

	dropCmd := &cobra.Command{
		Use:   "drop [name]",
		Short: "Drop a table",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doDelete("/api/tables/" + args[0] + "?db=" + dbName)
		},
	}
	dropCmd.Flags().StringVar(&dbName, "db", "test", "Database name")

	cmd.AddCommand(listCmd, createCmd, dropCmd)
	return cmd
}

func clusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster management commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show cluster status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doGet("/api/cluster/status")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "members",
		Short: "List cluster members",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doGet("/api/cluster/members")
		},
	})

	return cmd
}

func queryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query [json]",
		Short: "Execute a ReQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var query interface{}
			if err := json.Unmarshal([]byte(args[0]), &query); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			return doPost("/api/query", map[string]interface{}{"query": query})
		},
	}
}

func doGet(path string) error {
	resp, err := http.Get(apiURL + path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func doPost(path string, data interface{}) error {
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
		body = strings.NewReader(string(jsonData))
	}

	resp, err := http.Post(apiURL+path, "application/json", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func doDelete(path string) error {
	req, err := http.NewRequest("DELETE", apiURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func handleResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if output == "json" {
		fmt.Println(string(body))
	} else {
		// Pretty print JSON
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			fmt.Println(string(body))
			return nil
		}

		prettyJSON, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}

		fmt.Println(string(prettyJSON))
	}

	return nil
}
