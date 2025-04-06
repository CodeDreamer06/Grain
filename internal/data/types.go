package data

import "time"

// Log represents a single study or break entry.
type Log struct {
	Type      string    `json:"type"`      // "study" or "break"
	Timestamp time.Time `json:"timestamp"` // exact time
	Amount    int       `json:"amount"`    // e.g. +3 or -1
}

// Day aggregates logs for a specific calendar date.
type Day struct {
	Date string `json:"date"` // Format: "YYYY-MM-DD"
	Logs []Log  `json:"logs"`
}

// UndoItem stores the necessary information to revert a log action.
type UndoItem struct {
	Log     Log    `json:"log"`
	DayDate string `json:"day"` // The date string of the Day the log belonged to
}

// AppState holds the entire state of the application.
type AppState struct {
	Logs          []Day          `json:"logs"`           // Chronological list of days with logs
	WeeklySurplus map[string]int `json:"weekly_surplus"` // Key: "YYYY-WW", Value: surplus credits earned that week
	Streak        int            `json:"streak"`         // Current consecutive weeks meeting the goal
	BestSurplus   int            `json:"best_surplus"`   // Highest weekly surplus ever achieved
	UndoStack     []UndoItem     `json:"undo_stack"`     // Stack for undo operations
	Config        Config         `json:"-"`              // Runtime configuration, not saved in data.json
}

// Config holds user-specific settings.
type Config struct {
	WeeklyGoal int `json:"weekly_goal"` // Target study credits per week
	BreakStart int `json:"break_start"` // Break credits allocated at the start of each week
}

// Constants for log types
const (
	LogTypeStudy = "study"
	LogTypeBreak = "break"
)

// DateFormat defines the standard date format used throughout the app.
const DateFormat = "2006-01-02" // ISO 8601 format
