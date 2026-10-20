import { describe, expect, it } from 'vitest';
import { describeSchedule, describeLastResult } from '../src/lib/cockpitPresentation';

describe('cockpit presentation helpers', () => {
  it('turns common schedule presets into plain language', () => {
    expect(describeSchedule('hourly')).toBe('Every hour');
    expect(describeSchedule('daily')).toBe('Every day');
    expect(describeSchedule('0 */6 * * *')).toBe('Every 6 hours');
    expect(describeSchedule('*-*-* 06:00:00')).toBe('Every day at 6:00 AM');
    expect(describeSchedule('Mon *-*-* 07:00:00')).toBe('Every Monday at 7:00 AM');
    expect(describeSchedule('*-*-01 06:00:00')).toBe('Monthly on day 1 at 6:00 AM');
    expect(describeSchedule('')).toBeNull();
  });

  it('explains routine results in human terms', () => {
    expect(describeLastResult('success')).toBe('Completed successfully');
    expect(describeLastResult('failed')).toBe('The last run failed');
    expect(describeLastResult('ready')).toBeNull();
    expect(describeLastResult('custom result')).toBe('custom result');
  });
});
