package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/GoHyperrr/hyperrr/internal/scaffold"
	"github.com/spf13/cobra"
)

var (
	presetFlag   string
	dbFlag       string
	busFlag      string
	skipGitFlag  bool
	yesFlag      bool
)

var newCmd = &cobra.Command{
	Use:     "new <project-name>",
	Short:   "Scaffold a new Hyperrr project from template",
	Long:    `Scaffold a brand new Hyperrr commerce application project including directory structures, config file, default modules, environment templates, and local server setup.`,
	Args:    cobra.ExactArgs(1),
	GroupID: "engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		projectPath := projectName

		// Standardize project path
		absPath, err := filepath.Abs(projectPath)
		if err != nil {
			return err
		}

		// Check if directory already exists and is not empty
		if entries, err := os.ReadDir(absPath); err == nil && len(entries) > 0 {
			return fmt.Errorf("directory '%s' already exists and is not empty", projectPath)
		}

		// Defaults
		gitUser := getGitUser()
		defaultModulePath := fmt.Sprintf("github.com/%s/%s", gitUser, projectName)
		modulePath := defaultModulePath
		preset := presetFlag
		dbDriver := dbFlag
		eventBus := busFlag
		skipGit := skipGitFlag

		if !yesFlag {
			reader := bufio.NewReader(os.Stdin)
			fmt.Println("=== Hyperrr Project Generator ===")
			fmt.Println("Let's configure your new commerce engine project.")
			fmt.Println()

			// 1. Module Path
			modulePath = promptString(reader, "Go Module Path", defaultModulePath)

			// 2. Preset
			fmt.Println("\nAvailable Presets:")
			fmt.Println("  [1] commerce-full    (Default - Auth, Product, Cart, Order, Payments, Taxonomy, SEO, Notifications)")
			fmt.Println("  [2] commerce-minimal (Product, Cart, Order, Key Auth)")
			fmt.Println("  [3] auth-only        (Key Auth, Email/Password Auth)")
			fmt.Println("  [4] bare             (Empty setup - add modules manually)")
			presetChoice := promptString(reader, "Choose module preset (1-4)", "1")
			switch presetChoice {
			case "1":
				preset = "commerce-full"
			case "2":
				preset = "commerce-minimal"
			case "3":
				preset = "auth-only"
			case "4":
				preset = "bare"
			default:
				preset = "commerce-full"
			}

			// 3. Database
			dbDriver = promptString(reader, "Database driver (sqlite/postgres)", "sqlite")
			dbDriver = strings.ToLower(strings.TrimSpace(dbDriver))
			if dbDriver != "postgres" {
				dbDriver = "sqlite"
			}

			// 4. Event Bus
			eventBus = promptString(reader, "Event bus provider (inmem/nats)", "inmem")
			eventBus = strings.ToLower(strings.TrimSpace(eventBus))
			if eventBus != "nats" {
				eventBus = "inmem"
			}

			// 5. Git
			gitChoice := promptString(reader, "Initialize git repository? (y/n)", "y")
			gitChoice = strings.ToLower(strings.TrimSpace(gitChoice))
			if gitChoice == "n" || gitChoice == "no" {
				skipGit = true
			}
			fmt.Println()
		}

		// Deduce DSN
		dbDSN := projectName + ".db"
		if dbDriver == "postgres" {
			dbDSN = fmt.Sprintf("host=localhost port=5432 user=postgres password=postgres dbname=%s sslmode=disable", projectName)
		}

		scaffoldConfig := &scaffold.ScaffoldConfig{
			ProjectName:      projectName,
			ProjectPath:      projectPath,
			ModulePath:       modulePath,
			PresetName:       preset,
			DBDriver:         dbDriver,
			DBDSN:            dbDSN,
			EventBusProvider: eventBus,
			SkipGit:          skipGit,
		}

		return scaffold.Run(scaffoldConfig)
	},
}

func promptString(reader *bufio.Reader, label, defaultValue string) string {
	fmt.Printf("%s [%s]: ", label, defaultValue)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func getGitUser() string {
	cmd := exec.Command("git", "config", "github.user")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		user := strings.TrimSpace(out.String())
		if user != "" {
			return user
		}
	}

	cmd = exec.Command("git", "config", "user.name")
	out.Reset()
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		name := strings.TrimSpace(out.String())
		if name != "" {
			name = strings.ReplaceAll(strings.ToLower(name), " ", "")
			return name
		}
	}

	return "user"
}

func init() {
	newCmd.Flags().StringVar(&presetFlag, "preset", "commerce-full", "Module preset to install (commerce-full, commerce-minimal, auth-only, bare)")
	newCmd.Flags().StringVar(&dbFlag, "db", "sqlite", "Default database driver (sqlite, postgres)")
	newCmd.Flags().StringVar(&busFlag, "bus", "inmem", "Default event bus provider (inmem, nats)")
	newCmd.Flags().BoolVar(&skipGitFlag, "no-git", false, "Skip git initialization")
	newCmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Skip interactive prompts and use defaults")

	rootCmd.AddCommand(newCmd)
}
