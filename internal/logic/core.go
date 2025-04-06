package logic

import (
	"fmt"
	"sort"
	"time"

	"grain/internal/data"
	"grain/internal/timeutil"
)

// AddLog records a new study or break log.
func AddLog(state *data.AppState, logType string, amount int, timestamp time.Time) error {
	if timestamp.Weekday() == time.Sunday {
		return fmt.Errorf("logging is disabled on Sundays 🧘")
	}
	if amount <= 0 {
		return fmt.Errorf("log amount must be positive")
	}

	day := timeutil.GetOrCreateDayLogs(state, timestamp)

	newLog := data.Log{
		Type:      logType,
		Timestamp: timestamp,
		Amount:    amount,
	}

	day.Logs = append(day.Logs, newLog)
	// Ensure logs within the day are sorted by timestamp
	sort.SliceStable(day.Logs, func(i, j int) bool {
		return day.Logs[i].Timestamp.Before(day.Logs[j].Timestamp)
	})

	// Add to undo stack
	state.UndoStack = append(state.UndoStack, data.UndoItem{
		Log:     newLog,
		DayDate: day.Date,
	})

	// Recalculate stats after adding log
	RecalculateWeeklyStats(state, timeutil.GetWeekID(timestamp))

	return nil
}

// UndoLastAction reverts the most recent log action.
func UndoLastAction(state *data.AppState) (*data.Log, error) {
	if len(state.UndoStack) == 0 {
		return nil, fmt.Errorf("no actions to undo")
	}

	// Pop the last action from the undo stack
	lastUndoItem := state.UndoStack[len(state.UndoStack)-1]
	state.UndoStack = state.UndoStack[:len(state.UndoStack)-1]

	// Find the corresponding day log
	day, found := timeutil.GetDayLogs(state, lastUndoItem.DayDate)
	if !found {
		// This should theoretically not happen if data is consistent
		return nil, fmt.Errorf("internal error: cannot find day log '%s' for undo", lastUndoItem.DayDate)
	}

	// Find and remove the specific log entry from the day
	originalLogIndex := -1
	for i, log := range day.Logs {
		// Compare by timestamp and amount for uniqueness within the day
		if log.Timestamp.Equal(lastUndoItem.Log.Timestamp) && log.Amount == lastUndoItem.Log.Amount && log.Type == lastUndoItem.Log.Type {
			originalLogIndex = i
			break
		}
	}

	if originalLogIndex == -1 {
		// This should also not happen if the undo stack is correct
		return nil, fmt.Errorf("internal error: cannot find log entry to undo in day '%s'", lastUndoItem.DayDate)
	}

	// Remove the log entry
	day.Logs = append(day.Logs[:originalLogIndex], day.Logs[originalLogIndex+1:]...)

	// If the day becomes empty after removal, remove the day itself (optional, keeps data clean)
	if len(day.Logs) == 0 {
		RemoveDay(state, lastUndoItem.DayDate)
	}

	// Recalculate stats for the affected week
	undoneLogTime, _ := time.Parse(data.DateFormat, lastUndoItem.DayDate)
	RecalculateWeeklyStats(state, timeutil.GetWeekID(undoneLogTime))
	RecalculateOverallStats(state) // Recalculate overall stats like streak

	return &lastUndoItem.Log, nil
}

// RemoveDay removes a Day struct by its date string.
func RemoveDay(state *data.AppState, dateStr string) {
	newLogs := []data.Day{}
	for _, day := range state.Logs {
		if day.Date != dateStr {
			newLogs = append(newLogs, day)
		}
	}
	state.Logs = newLogs
}

// CalculateCurrentWeekStats computes study credits, break credits used, and available breaks for the current week.
func CalculateCurrentWeekStats(state *data.AppState) (studyCredits, breaksUsed, breaksAvailable int) {
	now := time.Now()
	startOfWeek, endOfWeek := timeutil.GetWeekBounds(now)
	weekID := timeutil.GetWeekID(now)

	studyCredits = 0
	breaksUsed = 0

	for _, day := range state.Logs {
		dayDate, err := time.Parse(data.DateFormat, day.Date)
		if err != nil {
			continue // Skip invalid date formats
		}

		// Check if the day falls within the current week (inclusive)
		if (dayDate.Equal(startOfWeek) || dayDate.After(startOfWeek)) && (dayDate.Equal(endOfWeek) || dayDate.Before(endOfWeek)) {
			if dayDate.Weekday() != time.Sunday { // Exclude Sunday
				for _, log := range day.Logs {
					if log.Type == data.LogTypeStudy {
						studyCredits += log.Amount
					} else if log.Type == data.LogTypeBreak {
						breaksUsed += log.Amount
					}
				}
			}
		}
	}

	// Calculate available breaks
	surplus := 0
	if studyCredits > state.Config.WeeklyGoal {
		surplus = (studyCredits - state.Config.WeeklyGoal) * 2
	}

	// Get surplus from the map, default to 0 if not present
	currentWeekSurplus := state.WeeklySurplus[weekID]
	if currentWeekSurplus < 0 {
		currentWeekSurplus = 0 // Ensure surplus isn't negative in calculation
	}

	// Available breaks = Starting breaks + Surplus earned this week - Breaks used
	breaksAvailable = state.Config.BreakStart + currentWeekSurplus - breaksUsed

	// Cap available breaks at the start-of-week amount if no surplus earned yet
	// This logic might need refinement depending on exact desired behavior with surplus carry-over
	if surplus == 0 && breaksAvailable > state.Config.BreakStart {
		// breaksAvailable = state.Config.BreakStart // Simple cap, adjust if needed
	} else if breaksAvailable < 0 {
		breaksAvailable = 0 // Cannot have negative available breaks
	}

	// Update state surplus map (ensure it reflects calculated surplus)
	if surplus != currentWeekSurplus {
		state.WeeklySurplus[weekID] = surplus
		// Check if this new surplus is the best ever
		if surplus > state.BestSurplus {
			state.BestSurplus = surplus
		}
	}

	return studyCredits, breaksUsed, breaksAvailable
}

// RecalculateWeeklyStats recalculates surplus for a specific week.
func RecalculateWeeklyStats(state *data.AppState, weekID string) {
	var weekStartTime time.Time
	fmt.Sscanf(weekID, "%d-%d", &weekStartTime)
	// Need to parse weekID back to a time to get bounds accurately
	year, weekNum := 0, 0
	_, err := fmt.Sscanf(weekID, "%d-%d", &year, &weekNum)
	if err != nil {
		fmt.Printf("Error parsing week ID '%s': %v\n", weekID, err)
		return
	}

	// Find a Monday within that ISO week and year
	t := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, 1)
	}
	t = t.AddDate(0, 0, (weekNum-1)*7)
	// Adjust if the first day calculation was off due to year boundary
	yCheck, wCheck := t.ISOWeek()
	if yCheck != year || wCheck != weekNum {
		// Try adjusting - this can be complex around year ends
		// A simpler approach might be needed if this fails often
		diff := (weekNum - wCheck)
		t = t.AddDate(0, 0, diff*7)
	}

	startOfWeek, endOfWeek := timeutil.GetWeekBounds(t)

	studyCredits := 0
	for _, day := range state.Logs {
		dayDate, err := time.Parse(data.DateFormat, day.Date)
		if err != nil {
			continue
		}
		if (dayDate.Equal(startOfWeek) || dayDate.After(startOfWeek)) && (dayDate.Equal(endOfWeek) || dayDate.Before(endOfWeek)) {
			if dayDate.Weekday() != time.Sunday {
				for _, log := range day.Logs {
					if log.Type == data.LogTypeStudy {
						studyCredits += log.Amount
					}
				}
			}
		}
	}

	surplus := 0
	if studyCredits >= state.Config.WeeklyGoal {
		surplus = (studyCredits - state.Config.WeeklyGoal) * 2
	}

	state.WeeklySurplus[weekID] = surplus
	if surplus > state.BestSurplus {
		state.BestSurplus = surplus
	}
}

// RecalculateOverallStats updates streak and potentially other long-term stats.
func RecalculateOverallStats(state *data.AppState) {
	now := time.Now()
	currentWeekID := timeutil.GetWeekID(now)
	currentStreak := 0

	// Iterate backwards from the week before the current one
	checkTime := now.AddDate(0, 0, -7)

	for {
		weekID := timeutil.GetWeekID(checkTime)
		if weekID == currentWeekID { // Should not happen with initial -7 days, but safety check
			break
		}

		// Calculate study credits for this past week
		startOfWeek, endOfWeek := timeutil.GetWeekBounds(checkTime)
		studyCredits := 0
		foundLogs := false
		for _, day := range state.Logs {
			dayDate, err := time.Parse(data.DateFormat, day.Date)
			if err != nil {
				continue
			}
			if (dayDate.Equal(startOfWeek) || dayDate.After(startOfWeek)) && (dayDate.Equal(endOfWeek) || dayDate.Before(endOfWeek)) {
				if dayDate.Weekday() != time.Sunday {
					foundLogs = true
					for _, log := range day.Logs {
						if log.Type == data.LogTypeStudy {
							studyCredits += log.Amount
						}
					}
				}
			}
		}

		// If no logs found for the week OR goal not met, streak breaks
		if !foundLogs || studyCredits < state.Config.WeeklyGoal {
			break
		}

		// Goal met for this week, increment streak
		currentStreak++

		// Move to the previous week
		checkTime = checkTime.AddDate(0, 0, -7)

		// Safety break: Avoid infinite loops if data is very old or sparse
		if len(state.Logs) > 0 && checkTime.Before(time.Now().AddDate(-5, 0, 0)) { // Check up to 5 years back
			break
		}
		if len(state.Logs) == 0 { // No logs, no streak
			break
		}
	}

	state.Streak = currentStreak
}

// CalculateTotalStats computes overall totals.
func CalculateTotalStats(state *data.AppState) (totalStudy, totalBreaks, totalEntries int) {
	for _, day := range state.Logs {
		for _, log := range day.Logs {
			totalEntries++
			if log.Type == data.LogTypeStudy {
				totalStudy += log.Amount
			} else if log.Type == data.LogTypeBreak {
				totalBreaks += log.Amount
			}
		}
	}
	return
}

// ResetWeekData clears logs for the current week and resets surplus.
func ResetWeekData(state *data.AppState) error {
	now := time.Now()
	startOfWeek, endOfWeek := timeutil.GetWeekBounds(now)
	currentWeekID := timeutil.GetWeekID(now)

	newLogs := []data.Day{}
	for _, day := range state.Logs {
		dayDate, err := time.Parse(data.DateFormat, day.Date)
		if err != nil {
			newLogs = append(newLogs, day) // Keep days with invalid dates? Or log error?
			continue
		}

		// Keep the day only if it's outside the current week
		if dayDate.Before(startOfWeek) || dayDate.After(endOfWeek) {
			newLogs = append(newLogs, day)
		}
	}
	state.Logs = newLogs

	// Reset surplus for the current week
	delete(state.WeeklySurplus, currentWeekID)

	// Clear undo stack as reset is a point of no return for the week's data
	state.UndoStack = []data.UndoItem{}

	// Recalculate overall stats as streak might be affected
	RecalculateOverallStats(state)

	return nil
}
