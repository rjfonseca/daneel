export function describeSchedule(schedule: string): string | null {
  const value = schedule.trim().toLowerCase();
  if (!value) return null;
  const presets: Record<string, string> = {
    hourly: 'Every hour',
    daily: 'Every day',
    weekly: 'Every week',
    monthly: 'Every month',
  };
  if (presets[value]) return presets[value];
  if (value === '0 * * * *') return 'Every hour';
  const everyHours = value.match(/^0 \*\/(\d+) \* \* \*$/);
  if (everyHours) return `Every ${everyHours[1]} hours`;

  const time = (hour: string, minute: string) => {
    const hourNumber = Number(hour);
    const period = hourNumber >= 12 ? 'PM' : 'AM';
    const displayHour = hourNumber % 12 || 12;
    return `${displayHour}:${minute} ${period}`;
  };
  const daily = value.match(/^\*-\*-\* (\d{2}):(\d{2}):\d{2}$/);
  if (daily) return `Every day at ${time(daily[1], daily[2])}`;

  const weekly = value.match(/^(Mon|Tue|Wed|Thu|Fri|Sat|Sun) \*-\*-\* (\d{2}):(\d{2}):\d{2}$/i);
  if (weekly) {
    const weekday = weekly[1][0].toUpperCase() + weekly[1].slice(1).toLowerCase();
    const fullName: Record<string, string> = { Mon: 'Monday', Tue: 'Tuesday', Wed: 'Wednesday', Thu: 'Thursday', Fri: 'Friday', Sat: 'Saturday', Sun: 'Sunday' };
    return `Every ${fullName[weekday]} at ${time(weekly[2], weekly[3])}`;
  }

  const monthly = value.match(/^\*-\*-(\d{2}) (\d{2}):(\d{2}):\d{2}$/);
  if (monthly) return `Monthly on day ${Number(monthly[1])} at ${time(monthly[2], monthly[3])}`;

  const everyMinutes = value.match(/^\*:([\d,]+):\d{2}$/);
  if (everyMinutes) {
    const minutes = everyMinutes[1].split(',').map(Number).sort((a, b) => a - b);
    if (minutes.length > 1 && minutes[0] === 0) {
      const interval = minutes[1] - minutes[0];
      if (interval > 0 && minutes.every((minute, index) => minute === index * interval)) {
        return `Every ${interval} minutes`;
      }
    }
    return 'Several times an hour';
  }

  return 'Custom schedule';
}

export function describeLastResult(result: string): string | null {
  const value = result.trim();
  if (!value || ['unknown', 'ready', 'not installed'].includes(value.toLowerCase())) return null;
  switch (value.toLowerCase()) {
    case 'success':
      return 'Completed successfully';
    case 'failed':
    case 'failure':
    case 'error':
      return 'The last run failed';
    case 'running':
      return 'Currently running';
    default:
      return value;
  }
}

export function formatRunTimestamp(unixSeconds: bigint | number): string | null {
  const seconds = Number(unixSeconds);
  if (!Number.isFinite(seconds) || seconds <= 0) return null;
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(seconds * 1000));
}
