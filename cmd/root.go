package cmd

import (
	"fmt"
	"os"
	"os/exec" // Added for config command
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"grain/internal/cli"
	"grain/internal/config"
	"grain/internal/data"
	"grain/internal/logic"
	"grain/internal/timeutil"

	"github.com/spf13/cobra"
)

var (
	cfg        data.Config
	appState   data.AppState
	baseDir    string
	configPath string
	dataPath   string
	backupDir  string
	errLog     func(err error) // Simplified error handling
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "grain [amount]",
	Short: "🧘 Grain: Minimalist habit tracker for focused work & breaks.",
	Long: `Grain helps you track focused work (study) and mindful breaks 
using a simple credit system. Log study time to earn credits, 
and spend them on breaks. Simple, local, and calm.`,
	Args: cobra.MaximumNArgs(1), // Allow 0 or 1 argument (for the amount)
	// SilenceUsage: true, // Prevents usage message on error handled by errLog
	Run: func(cmd *cobra.Command, args []string) {
		amount := 1 // Default amount
		var err error
		if len(args) == 1 {
			amount, err = strconv.Atoi(args[0])
			if err != nil || amount <= 0 {
				errLog(fmt.Errorf("invalid amount: '%s'. Please provide a positive number", args[0]))
				return // errLog exits, but return for clarity
			}
		}
		// Default action: log study credits
		if err := logic.AddLog(&appState, data.LogTypeStudy, amount, time.Now()); err != nil {
			errLog(err)
			return
		}
		if err := data.SaveState(dataPath, &appState); err != nil {
			errLog(err)
			return
		}
		fmt.Printf("✨ +%d study credits logged. Keep it rolling!\n", amount)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// init performs setup before Cobra executes commands.
func init() {
	// Initialize error logger
	errLog = func(err error) {
		cli.PrintError(err) // Print using our formatted error func
		os.Exit(1)
	}

	cobra.OnInitialize(loadConfigAndState) // Use Cobra's initialization hook
	addCommands()                          // Add commands after initialization setup
}

// loadConfigAndState loads the application configuration and data state.
// It's called by cobra.OnInitialize.
func loadConfigAndState() {
	var err error
	baseDir, configPath, dataPath, backupDir, err = config.GetPaths()
	if err != nil {
		errLog(fmt.Errorf("initialization error creating directories: %w", err))
	}

	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		errLog(fmt.Errorf("failed to load config: %w", err))
	}

	// Check if data file exists before loading state
	firstRun := false
	if _, statErr := os.Stat(dataPath); os.IsNotExist(statErr) {
		firstRun = true
	}

	appState, err = data.LoadState(dataPath, cfg) // Pass loaded config to state
	if err != nil {
		errLog(fmt.Errorf("failed to load state: %w", err))
	}

	// Perform initial calculations or ensure stats are up-to-date
	logic.RecalculateOverallStats(&appState) // Recalculate streak, best surplus based on loaded data
	// No need to explicitly save here unless firstRun caused changes needing immediate persistence
	// Save operations happen within commands after modification.
	if firstRun {
		// Save the initialized state if it was the very first run
		if err := data.SaveState(dataPath, &appState); err != nil {
			errLog(fmt.Errorf("failed to save initial state: %w", err))
		}
	}
}

// addCommands registers all subcommands to the root command.
func addCommands() {
	// Define flags
	var sinceFlag string

	// --- Add Study/Break Logging Commands ---
	studyCmd := &cobra.Command{
		Use:     "s [amount]",
		Aliases: []string{"study"},
		Short:   "🧠 Log study credits (default: 1)",
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			amount := 1
			var err error
			if len(args) == 1 {
				amount, err = strconv.Atoi(args[0])
				if err != nil || amount <= 0 {
					errLog(fmt.Errorf("invalid amount: '%s'", args[0]))
					return
				}
			}
			if err := logic.AddLog(&appState, data.LogTypeStudy, amount, time.Now()); err != nil {
				errLog(err)
				return
			}
			if err := data.SaveState(dataPath, &appState); err != nil {
				errLog(err)
				return
			}
			fmt.Printf("✨ +%d study credits logged. Keep it rolling!\n", amount)
		},
	}

	breakCmd := &cobra.Command{
		Use:     "b [amount]",
		Aliases: []string{"break"},
		Short:   "🍵 Log break credits (default: 1)",
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			amount := 1
			var err error
			if len(args) == 1 {
				amount, err = strconv.Atoi(args[0])
				if err != nil || amount <= 0 {
					errLog(fmt.Errorf("invalid amount: '%s'", args[0]))
					return
				}
			}

			// Check if enough break credits are available before logging
			_, _, breaksAvailable := logic.CalculateCurrentWeekStats(&appState)
			if amount > breaksAvailable {
				errLog(fmt.Errorf("not enough break credits (need %d, have %d)", amount, breaksAvailable))
				return
			}

			if err := logic.AddLog(&appState, data.LogTypeBreak, amount, time.Now()); err != nil {
				errLog(err)
				return
			}
			if err := data.SaveState(dataPath, &appState); err != nil {
				errLog(err)
				return
			}
			fmt.Printf("🍵 -%d break credit logged. Breathe easy.\n", amount)
		},
	}

	rootCmd.AddCommand(studyCmd)
	rootCmd.AddCommand(breakCmd)

	// --- Add View Commands ---
	logCmd := &cobra.Command{
		Use:   "log",
		Short: "🗓️  View log entries",
		Long:  "View log entries. By default, shows today. Use --since to specify a start date (e.g., 'yesterday', 'monday', 'YYYY-MM-DD').",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			now := time.Now()
			startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()) // Default to start of today
			endDate := startDate.Add(24 * time.Hour)                                               // Default to end of today

			headerDateStr := now.Format("Jan 2")

			if sinceFlag != "" {
				sinceFlag = strings.ToLower(sinceFlag)
				if sinceFlag == "today" {
					// Already defaulted to today
				} else if sinceFlag == "yesterday" {
					startDate = startDate.AddDate(0, 0, -1)
					headerDateStr = startDate.Format("Jan 2")
					endDate = startDate.Add(24 * time.Hour)
				} else if sinceFlag == "monday" {
					startOfWeek, _ := timeutil.GetWeekBounds(now)
					startDate = startOfWeek
					headerDateStr = fmt.Sprintf("Week of %s", startDate.Format("Jan 2"))
					endDate = now // Show up to current time if filtering from Monday
				} else {
					// Try parsing as YYYY-MM-DD
					parsedDate, err := time.Parse(data.DateFormat, sinceFlag)
					if err != nil {
						errLog(fmt.Errorf("invalid --since value: '%s'. Use 'today', 'yesterday', 'monday', or 'YYYY-MM-DD'", sinceFlag))
						return
					}
					startDate = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, now.Location())
					headerDateStr = fmt.Sprintf("Since %s", startDate.Format("Jan 2"))
					endDate = now // Show up to current time if filtering from a specific date
				}
			}

			fmt.Println(cli.FormatHeader(fmt.Sprintf("🗓️  Log %s", headerDateStr)))

			foundLogs := false
			totalStudy := 0
			totalBreaks := 0

			// Iterate through all days and logs, filtering by date range
			for _, day := range appState.Logs {
				dayDate, err := time.Parse(data.DateFormat, day.Date)
				if err != nil {
					continue // Skip malformed dates in data
				}

				// Check if the day is within the filter range [startDate, endDate)
				if (dayDate.Equal(startDate) || dayDate.After(startDate)) && dayDate.Before(endDate) {
					for _, log := range day.Logs {
						// Also check log timestamp is within range (useful for multi-day filters)
						if (log.Timestamp.Equal(startDate) || log.Timestamp.After(startDate)) && log.Timestamp.Before(endDate) {
							fmt.Println(cli.FormatLogEntry(log))
							foundLogs = true
							if log.Type == data.LogTypeStudy {
								totalStudy += log.Amount
							} else {
								totalBreaks += log.Amount
							}
						}
					}
				}
			}

			if !foundLogs {
				fmt.Println("No matching entries found.")
				return
			}

			fmt.Printf("\nTotal ▸ 🧠 %d study   💤 %d break\n", totalStudy, totalBreaks)
		},
	}
	// Add the flag to the log command
	logCmd.Flags().StringVar(&sinceFlag, "since", "", "Show logs since a specific time (e.g., 'today', 'yesterday', 'monday', 'YYYY-MM-DD')")

	weekCmd := &cobra.Command{
		Use:   "week",
		Short: "📊 View current weekly overview",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			now := time.Now()
			startOfWeek, _ := timeutil.GetWeekBounds(now)
			// Recalculate just before display to ensure freshness
			studyCredits, _, breaksAvailable := logic.CalculateCurrentWeekStats(&appState)
			logic.RecalculateOverallStats(&appState) // Ensure streak is also fresh

			fmt.Println(cli.FormatHeader(fmt.Sprintf("📊 Week of %s", startOfWeek.Format("Jan 2"))))
			fmt.Printf("🧠 Study     ▸ %d / %d\n", studyCredits, appState.Config.WeeklyGoal)
			fmt.Printf("💤 Breaks    ▸ %d / %d\n", breaksAvailable, appState.Config.BreakStart)

			// Explicitly get current week surplus from the map
			currentWeekID := timeutil.GetWeekID(now)
			currentSurplus := appState.WeeklySurplus[currentWeekID]
			// Ensure surplus calculation matches expectation (non-negative, based on goal)
			if studyCredits >= appState.Config.WeeklyGoal {
				calculatedSurplus := (studyCredits - appState.Config.WeeklyGoal) * 2
				// Use calculated surplus if different from stored, though CalculateCurrentWeekStats should update it.
				// This is more for display consistency.
				currentSurplus = calculatedSurplus
			} else {
				currentSurplus = 0 // Don't display negative surplus
			}

			fmt.Printf("✨ Surplus   ▸ %d\n", currentSurplus)
			fmt.Printf("🔥 Streak    ▸ %d weeks\n", appState.Streak)
		},
	}

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "📈 Show overall stats (streak, totals)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			// Ensure stats are fresh before displaying
			logic.RecalculateOverallStats(&appState)
			totalStudy, totalBreaks, totalEntries := logic.CalculateTotalStats(&appState)

			fmt.Println(cli.FormatHeader("📈 Your Stats"))
			fmt.Printf("🔁 Streak:         %d weeks\n", appState.Streak)
			fmt.Printf("🏆 Best Surplus:   +%d\n", appState.BestSurplus)
			fmt.Printf("📚 Total Study:    %d credits\n", totalStudy)
			fmt.Printf("🍵 Total Breaks:   %d credits\n", totalBreaks)
			fmt.Printf("🧾 Total Entries:  %d\n", totalEntries)
		},
	}

	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(weekCmd)
	rootCmd.AddCommand(statsCmd)

	// --- Add Goal Command ---
	goalCmd := &cobra.Command{
		Use:   "goal [amount]",
		Short: "🎯 View or set your weekly study goal",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				// View current goal
				fmt.Printf("🎯 Current weekly study goal: %d credits\n", appState.Config.WeeklyGoal)
				return
			}

			// Set new goal
			newGoal, err := strconv.Atoi(args[0])
			if err != nil || newGoal <= 0 {
				errLog(fmt.Errorf("invalid goal amount: '%s'. Please provide a positive number", args[0]))
				return
			}

			// Update the config in memory
			appState.Config.WeeklyGoal = newGoal

			// Save the updated config to file
			if err := config.SaveConfig(configPath, appState.Config); err != nil {
				// Attempt to restore old value in memory if save fails?
				// For simplicity now, just log error. User might need to fix file permissions.
				errLog(fmt.Errorf("failed to save updated config file: %w", err))
				return
			}

			fmt.Printf("🎯 Weekly study goal updated to: %d credits\n", newGoal)
		},
	}
	rootCmd.AddCommand(goalCmd)

	// --- Add Action Commands ---
	undoCmd := &cobra.Command{
		Use:   "undo",
		Short: "🔙 Undoes the last logged action",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			undoneLog, err := logic.UndoLastAction(&appState)
			if err != nil {
				errLog(err)
				return
			}
			if err := data.SaveState(dataPath, &appState); err != nil {
				errLog(err)
				return
			}
			fmt.Printf("🔙 Undid log: %s\n", cli.FormatLogEntry(*undoneLog))
			fmt.Printf("Remaining undo steps: %d\n", len(appState.UndoStack))
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "⚙️  Opens config file in your default editor ($EDITOR)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				// Simple default lookup
				if _, err := exec.LookPath("vim"); err == nil {
					editor = "vim"
				} else if _, err := exec.LookPath("nano"); err == nil {
					editor = "nano"
				} else if _, err := exec.LookPath("code"); err == nil { // Common VS Code
					editor = "code"
				} else {
					errLog(fmt.Errorf("EDITOR environment variable not set and common editors (vim, nano, code) not found.\nPlease edit manually: %s", configPath))
					return
				}
			}

			fmt.Printf("Attempting to open %s with %s...\n", configPath, editor)
			editorCmd := exec.Command(editor, configPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				errLog(fmt.Errorf("failed to open editor '%s': %w\nCheck if '%s' is in your PATH.", editor, err, editor))
				return
			}
			fmt.Println("Editor closed. Configuration changes will be applied the next time you run grain.")
		},
	}

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "🧹 Prompts to reset data for the current week",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			// Use the specific confirmation string required
			if cli.PromptConfirmation("⚠️  Are you sure you want to reset this week's data?\nType \"reset grain\" to confirm:") {
				if err := logic.ResetWeekData(&appState); err != nil {
					errLog(fmt.Errorf("failed to reset week data: %w", err))
					return
				}
				if err := data.SaveState(dataPath, &appState); err != nil {
					errLog(err)
					return
				}
				fmt.Println("🧹 Current week data has been reset.")
			} else {
				fmt.Println("Reset cancelled.")
			}
		},
	}

	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "🗃️ Saves a timestamped backup of all data",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			backupFile, err := data.BackupData(dataPath, backupDir)
			if err != nil {
				errLog(err)
				return
			}
			// Use relative path for display if possible
			relBackupPath, err := filepath.Rel(baseDir, backupFile)
			if err == nil {
				backupFile = filepath.Join("~/.grain", relBackupPath)
			}
			fmt.Printf("🗃️ Backup saved to: %s\n", backupFile)
		},
	}

	restoreCmd := &cobra.Command{
		Use:   "restore <backup_file_name>",
		Short: "♻️  Loads state from a backup file in ~/.grain/backups/",
		Long: `Restores the application state from a specified backup file. 
The backup file name should exist within the ~/.grain/backups/ directory. 
This action will overwrite your current data.json file.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			backupFileName := args[0]
			// Ensure the provided name doesn't contain path separators
			if filepath.Base(backupFileName) != backupFileName {
				errLog(fmt.Errorf("invalid backup file name: '%s'. Please provide only the filename, not a path.", backupFileName))
				return
			}
			backupFilePath := filepath.Join(backupDir, backupFileName)

			// Use a simple 'yes' confirmation for restore
			if cli.PromptConfirmation(fmt.Sprintf("⚠️ This will overwrite current data with the contents of '%s'.\nType \"yes\" to confirm:", backupFileName)) {
				if err := data.RestoreData(dataPath, backupFilePath); err != nil {
					errLog(err)
					return
				}
				// Reload state immediately after restore to reflect changes
				loadConfigAndState() // Reloads cfg and appState, recalculates stats

				// We need to save the reloaded and recalculated state
				if err := data.SaveState(dataPath, &appState); err != nil {
					errLog(fmt.Errorf("failed to save state after restore: %w", err))
					return
				}

				fmt.Printf("♻️ Data restored from %s and current stats recalculated.\n", backupFileName)
			} else {
				fmt.Println("Restore cancelled.")
			}
		},
	}

	rootCmd.AddCommand(undoCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
}
