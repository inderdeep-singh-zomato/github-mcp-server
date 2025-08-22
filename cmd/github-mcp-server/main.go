package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/pkg/access"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// These variables are set by the build process using ldflags.
var version = "version"
var commit = "commit"
var date = "date"

var (
	rootCmd = &cobra.Command{
		Use:     "server",
		Short:   "GitHub MCP Server",
		Long:    `A GitHub MCP server that handles various tools and resources.`,
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio server",
		Long:  `Start a server that communicates via standard input/output streams using JSON-RPC messages.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			userEmail := viper.GetString("user_email")
			if userEmail == "" {
				// Check environment variable as fallback
				userEmail = os.Getenv("GITHUB_USER_EMAIL")
				if userEmail == "" {
					return errors.New("USER_EMAIL not provided in input or environment variable GITHUB_USER_EMAIL")
				}
			}

			token := viper.GetString("personal_access_token")
			if token == "" {
				// Fallback to environment variable
				token = os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
				if token == "" {
					return errors.New("GITHUB_PERSONAL_ACCESS_TOKEN not provided in input or environment")
				}
			}

			// If you're wondering why we're not using viper.GetStringSlice("toolsets"),
			// it's because viper doesn't handle comma-separated values correctly for env
			// vars when using GetStringSlice.
			// https://github.com/spf13/viper/issues/380
			var enabledToolsets []string
			if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
				return fmt.Errorf("failed to unmarshal toolsets: %w", err)
			}

			stdioServerConfig := ghmcp.StdioServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Token:                token,
				UserEmail:            userEmail,
				EnabledToolsets:      enabledToolsets,
				DynamicToolsets:      viper.GetBool("dynamic_toolsets"),
				ReadOnly:             viper.GetBool("read-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
			}
			return ghmcp.RunStdioServer(stdioServerConfig)
		},
	}

	validateAccessCmd = &cobra.Command{
		Use:   "validate-access",
		Short: "Validate repository access for a user",
		Long:  `Validate whether a user has access to a specific repository based on their email and the repository URL.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			
			// Step 1: Extract and validate user-email flag
			userEmail, err := cmd.Flags().GetString("user-email")
			if err != nil {
				return fmt.Errorf("failed to get user-email flag: %w", err)
			}
			if userEmail == "" {
				return errors.New("user-email is required")
			}

			// Step 2: Extract and validate repo-url flag
			repoURL, err := cmd.Flags().GetString("repo-url")
			if err != nil {
				return fmt.Errorf("failed to get repo-url flag: %w", err)
			}
			if repoURL == "" {
				return errors.New("repo-url is required")
			}

			// Step 3: Create and initialize validator
			validator := access.NewValidator(userEmail)
			
			if err := validator.Initialize(); err != nil {
				return fmt.Errorf("failed to initialize validator: %w", err)
			}

			// Step 4: Check repository access
			hasAccess, err := validator.IsRepositoryAccessible(repoURL)
			if err != nil {
				return fmt.Errorf("failed to validate repository access: %w", err)
			}

			// Step 5: Output result
			if hasAccess {
				fmt.Println("{\"hasAccess\": true}")
			} else {
				fmt.Println("{\"hasAccess\": false}")
			}

			return nil
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.SetGlobalNormalizationFunc(wordSepNormalizeFunc)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	// Add global flags that will be shared by all commands
	rootCmd.PersistentFlags().StringSlice("toolsets", github.DefaultTools, "An optional comma separated list of groups of tools to allow, defaults to enabling all")
	rootCmd.PersistentFlags().Bool("dynamic-toolsets", false, "Enable dynamic toolsets")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().String("log-file", "", "Path to log file")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().Bool("export-translations", false, "Save translations to a JSON file")
	rootCmd.PersistentFlags().String("gh-host", "", "Specify the GitHub hostname (for GitHub Enterprise etc.)")
	rootCmd.PersistentFlags().String("user-email", "", "User email for repository access validation (fallback: GITHUB_USER_EMAIL env var)")
	rootCmd.PersistentFlags().Int("content-window-size", 5000, "Specify the content window size")

	// Add command-specific flags for validate-access command
	validateAccessCmd.Flags().String("user-email", "", "User email for repository access validation (required)")
	validateAccessCmd.Flags().String("repo-url", "", "Repository URL to validate access for (required)")
	validateAccessCmd.MarkFlagRequired("user-email")
	validateAccessCmd.MarkFlagRequired("repo-url")

	// Bind flag to viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("dynamic_toolsets", rootCmd.PersistentFlags().Lookup("dynamic-toolsets"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))
	_ = viper.BindPFlag("user_email", rootCmd.PersistentFlags().Lookup("user-email"))
	_ = viper.BindPFlag("content-window-size", rootCmd.PersistentFlags().Lookup("content-window-size"))

	// Add subcommands
	rootCmd.AddCommand(stdioCmd)
	rootCmd.AddCommand(validateAccessCmd)
}

func initConfig() {
	// Initialize Viper configuration
	viper.SetEnvPrefix("github")
	viper.AutomaticEnv()

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func wordSepNormalizeFunc(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	from := []string{"_"}
	to := "-"
	for _, sep := range from {
		name = strings.ReplaceAll(name, sep, to)
	}
	return pflag.NormalizedName(name)
}
