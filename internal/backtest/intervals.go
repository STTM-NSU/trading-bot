package backtest

import "time"

type WeekInterval struct {
	Start time.Time
	End   time.Time
}

// SplitIntoWeeks разбивает интервал на недельные интервалы
func SplitIntoWeeks(from, to time.Time) []WeekInterval {
	var intervals []WeekInterval

	current := from.Truncate(24 * time.Hour)
	end := to.Truncate(24 * time.Hour)

	if current.After(end) {
		return intervals
	}

	current = findNextMonday(current)
	if current.After(end) {
		intervals = append(intervals, WeekInterval{
			Start: from,
			End:   end.Add(24*time.Hour - time.Nanosecond),
		})
		return intervals
	}

	firstSunday := current.Add(-24 * time.Hour)
	if firstSunday.After(from) {
		intervals = append(intervals, WeekInterval{
			Start: from,
			End:   firstSunday.Add(24*time.Hour - time.Nanosecond),
		})
	}

	for {
		nextSunday := current.Add(6 * 24 * time.Hour)
		if nextSunday.After(end) {
			break
		}

		intervals = append(intervals, WeekInterval{
			Start: current,
			End:   nextSunday.Add(24*time.Hour - time.Nanosecond),
		})

		current = current.Add(7 * 24 * time.Hour)
	}

	if current.Before(end) || current.Equal(end) {
		intervals = append(intervals, WeekInterval{
			Start: current,
			End:   end.Add(24*time.Hour - time.Nanosecond),
		})
	}

	return intervals
}

func findNextMonday(t time.Time) time.Time {
	weekday := t.Weekday()
	daysUntilMonday := (8 - int(weekday)) % 7
	return t.AddDate(0, 0, daysUntilMonday)
}

func DivideIntoHours(from, to time.Time) []time.Time {
	hours := make([]time.Time, 0, int(to.Sub(from).Hours()))
	for from.Before(to) {
		hours = append(hours, from)
		from = from.Add(1 * time.Hour)
	}

	return hours
}
