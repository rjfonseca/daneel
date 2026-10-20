package routine

import "testing"

func TestParseScheduleToOnCalendar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Presets
		{name: "preset hourly", input: "hourly", expected: "hourly"},
		{name: "preset daily", input: "daily", expected: "*-*-* 06:00:00"},
		{name: "preset weekly", input: "weekly", expected: "Mon *-*-* 07:00:00"},
		{name: "preset monthly", input: "monthly", expected: "*-*-01 06:00:00"},

		// Already valid systemd OnCalendar strings
		{name: "systemd direct calendar", input: "*-*-* 00,06,12,18:00:00", expected: "*-*-* 00,06,12,18:00:00"},
		{name: "systemd range calendar", input: "Mon..Fri *-*-* 09:00:00", expected: "Mon..Fri *-*-* 09:00:00"},
		{name: "systemd date specific", input: "2026-01-01 00:00:00", expected: "2026-01-01 00:00:00"},

		// 5-field standard cron expressions
		{name: "cron every 6 hours", input: "0 */6 * * *", expected: "*-*-* 00,06,12,18:00:00"},
		{name: "cron exact time", input: "0 6 * * *", expected: "*-*-* 06:00:00"},
		{name: "cron every 30 minutes", input: "*/30 * * * *", expected: "*:00,30:00"},
		{name: "cron specific hour and minute", input: "15 10 * * *", expected: "*-*-* 10:15:00"},
		{name: "cron sunday midnight", input: "0 0 * * 0", expected: "Sun *-*-* 00:00:00"},
		{name: "cron sunday 7 midnight", input: "0 0 * * 7", expected: "Sun *-*-* 00:00:00"},
		{name: "cron weekday range", input: "30 2 * * 1-5", expected: "Mon..Fri *-*-* 02:30:00"},
		{name: "cron comma hours", input: "0 8,12,16 * * *", expected: "*-*-* 08,12,16:00:00"},
		{name: "cron every 15 minutes", input: "*/15 * * * *", expected: "*:00,15,30,45:00"},
		{name: "cron every 2 hours", input: "0 */2 * * *", expected: "*-*-* 00,02,04,06,08,10,12,14,16,18,20,22:00:00"},

		// Fallback / edge cases
		{name: "empty string", input: "", expected: "daily"},
		{name: "whitespace padded", input: "  hourly  ", expected: "hourly"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseScheduleToOnCalendar(tt.input)
			if got != tt.expected {
				t.Errorf("ParseScheduleToOnCalendar(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
