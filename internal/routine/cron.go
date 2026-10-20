package routine

import (
	"fmt"
	"strconv"
	"strings"
)

var dayOfWeekMap = map[string]string{
	"0": "Sun",
	"1": "Mon",
	"2": "Tue",
	"3": "Wed",
	"4": "Thu",
	"5": "Fri",
	"6": "Sat",
	"7": "Sun",
}

// ParseScheduleToOnCalendar converts friendly schedule presets or standard 5-field cron
// expressions into systemd OnCalendar format.
func ParseScheduleToOnCalendar(schedule string) string {
	s := strings.TrimSpace(schedule)
	if s == "" {
		return "daily"
	}

	lower := strings.ToLower(s)
	switch lower {
	case "hourly":
		return "hourly"
	case "daily":
		return "*-*-* 06:00:00"
	case "weekly":
		return "Mon *-*-* 07:00:00"
	case "monthly":
		return "*-*-01 06:00:00"
	}

	// If already in systemd calendar format or explicit timestamp
	if strings.Contains(s, "*-*-*") || strings.Contains(s, "*:*:*") ||
		strings.Contains(s, "..") || strings.HasPrefix(s, "20") {
		return s
	}

	fields := strings.Fields(s)
	if len(fields) != 5 {
		return s
	}

	minField, hourField, domField, monthField, dowField := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Handle case: */N * * * * (e.g. */30 * * * *)
	if hourField == "*" && domField == "*" && monthField == "*" && dowField == "*" && strings.HasPrefix(minField, "*/") {
		stepStr := strings.TrimPrefix(minField, "*/")
		if step, err := strconv.Atoi(stepStr); err == nil && step > 0 && step < 60 {
			var mins []string
			for m := 0; m < 60; m += step {
				mins = append(mins, fmt.Sprintf("%02d", m))
			}
			return fmt.Sprintf("*:%s:00", strings.Join(mins, ","))
		}
	}

	// Expand minutes
	minPart := expandNumberList(minField, 59)
	if minPart == "" {
		minPart = "00"
	}

	// Expand hours
	hourPart := expandNumberList(hourField, 23)
	if hourPart == "" {
		hourPart = "00"
	}

	// Resolve Day of Week
	dowPart := ""
	if dowField != "*" {
		if strings.Contains(dowField, "-") {
			parts := strings.Split(dowField, "-")
			if len(parts) == 2 {
				startDay := dayOfWeekMap[parts[0]]
				endDay := dayOfWeekMap[parts[1]]
				if startDay != "" && endDay != "" {
					dowPart = fmt.Sprintf("%s..%s ", startDay, endDay)
				}
			}
		} else if day, ok := dayOfWeekMap[dowField]; ok {
			dowPart = fmt.Sprintf("%s ", day)
		}
	}

	// Date part
	datePart := "*-*-*"
	if domField != "*" || monthField != "*" {
		mStr := monthField
		if mStr == "*" {
			mStr = "*"
		} else if m, err := strconv.Atoi(mStr); err == nil {
			mStr = fmt.Sprintf("%02d", m)
		}

		dStr := domField
		if dStr == "*" {
			dStr = "*"
		} else if d, err := strconv.Atoi(dStr); err == nil {
			dStr = fmt.Sprintf("%02d", d)
		}
		datePart = fmt.Sprintf("*-%s-%s", mStr, dStr)
	}

	return fmt.Sprintf("%s%s %s:%s:00", dowPart, datePart, hourPart, minPart)
}

func expandNumberList(field string, maxVal int) string {
	if field == "*" {
		return "*"
	}

	if strings.HasPrefix(field, "*/") {
		stepStr := strings.TrimPrefix(field, "*/")
		if step, err := strconv.Atoi(stepStr); err == nil && step > 0 {
			var vals []string
			for v := 0; v <= maxVal; v += step {
				vals = append(vals, fmt.Sprintf("%02d", v))
			}
			return strings.Join(vals, ",")
		}
	}

	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		var vals []string
		for _, p := range parts {
			if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				vals = append(vals, fmt.Sprintf("%02d", v))
			}
		}
		if len(vals) > 0 {
			return strings.Join(vals, ",")
		}
	}

	if v, err := strconv.Atoi(field); err == nil {
		return fmt.Sprintf("%02d", v)
	}

	return field
}
