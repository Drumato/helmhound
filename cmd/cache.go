package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewCacheCommand creates the cache command with subcommands
func NewCacheCommand() *cobra.Command {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage helmhound cache",
	}

	cacheCmd.AddCommand(newCacheListCommand())

	return cacheCmd
}

// newCacheListCommand creates the cache list subcommand
func newCacheListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cached charts",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %v", err)
			}

			helmhoundDir := filepath.Join(homeDir, ".helmhound")

			// Check if cache directory exists
			if _, err := os.Stat(helmhoundDir); os.IsNotExist(err) {
				fmt.Println("No cached charts found. Cache directory does not exist.")
				return nil
			}

			// Read directory contents
			entries, err := os.ReadDir(helmhoundDir)
			if err != nil {
				return fmt.Errorf("failed to read cache directory: %v", err)
			}

			if len(entries) == 0 {
				fmt.Println("No cached charts found.")
				return nil
			}

			fmt.Println("Cached charts:")
			for _, entry := range entries {
				if entry.IsDir() {
					fmt.Printf("  - %s\n", entry.Name())
				}
			}

			return nil
		},
	}
}
