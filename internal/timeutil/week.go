package timeutil

import (
	"fmt"
	"time"

	"grain/internal/data"
)

// GetWeekBounds returns the start (Monday) and end (Sunday) dates for the week containing the given time.
func GetWeekBounds(t time.Time) (start, end time.Time) {
	// Go's Weekday starts with Sunday=0, Monday=1, ..., Saturday=6
	weekday := t.Weekday()
	// Adjust to make Monday the start of the week (Monday=0, ..., Sunday=6)
	adjWeekday := int(weekday) - 1
	if adjWeekday < 0 {
		adjWeekday = 6 // Sunday
	}

	// Calculate the start of the week (Monday)
	start = t.AddDate(0, 0, -adjWeekday)
	// Calculate the end of the week (Sunday)
	end = start.AddDate(0, 0, 6)

	// Ensure we are using the date part only, zeroing out time
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, t.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, t.Location())

	return start, end
}

// GetWeekID generates a unique string identifier for the week containing the given time (e.g., "2024-23").
func GetWeekID(t time.Time) string {
	year, week := t.ISOWeek() // ISO week starts on Monday
	return fmt.Sprintf("%d-%02d", year, week)
}

// GetCurrentWeekID returns the week ID for the current time.
func GetCurrentWeekID() string {
	return GetWeekID(time.Now())
}

// GetDayLogs retrieves the logs for a specific date string (YYYY-MM-DD).
// Returns the Day struct and a boolean indicating if it was found.
func GetDayLogs(state *data.AppState, dateStr string) (*data.Day, bool) {
	for i := range state.Logs {
		if state.Logs[i].Date == dateStr {
			return &state.Logs[i], true
		}
	}
	return nil, false
}

// GetOrCreateDayLogs finds or creates a Day struct for the given date.
// Ensures the Logs slice is sorted chronologically by date.
func GetOrCreateDayLogs(state *data.AppState, date time.Time) *data.Day {
	dateStr := date.Format(data.DateFormat)

	// Check if the day already exists
	day, found := GetDayLogs(state, dateStr)
	if found {
		return day
	}

	// Day doesn't exist, create it
	newDay := data.Day{
		Date: dateStr,
		Logs: []data.Log{},
	}

	// Insert the new day while maintaining sorted order
	insertIndex := 0
	for i, existingDay := range state.Logs {
		if existingDay.Date > dateStr {
			break
		}
		insertIndex = i + 1
	}

	if insertIndex == len(state.Logs) {
		state.Logs = append(state.Logs, newDay)
	} else {
		state.Logs = append(state.Logs[:insertIndex], append([]data.Day{newDay}, state.Logs[insertIndex:]...)...)
	}

	// Return the pointer to the newly added day in the slice
	return &state.Logs[insertIndex]
}
