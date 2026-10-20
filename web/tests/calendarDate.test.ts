import { describe, expect, it } from 'vitest';
import { addCalendarDays, getISOWeekInfo, getMonday, parseLocalDateString, toLocalDateString } from '../src/lib/calendarDate';

describe('calendar date helpers', () => {
  it('moves between weeks across year boundaries using calendar days', () => {
    const monday = getMonday(new Date(2026, 11, 28));
    expect(toLocalDateString(monday)).toBe('2026-12-28');
    expect(toLocalDateString(addCalendarDays(monday, 7))).toBe('2027-01-04');
    expect(getISOWeekInfo(addCalendarDays(monday, 7))).toEqual({ week: 1, year: 2027 });
  });

  it('formats local dates without UTC day shifts', () => {
    expect(toLocalDateString(new Date(2026, 2, 8, 0, 30))).toBe('2026-03-08');
    expect(toLocalDateString(new Date(2026, 10, 1, 23, 30))).toBe('2026-11-01');
  });

  it('parses only valid local calendar dates without UTC conversion', () => {
    expect(toLocalDateString(parseLocalDateString('2026-02-28')!)).toBe('2026-02-28');
    expect(parseLocalDateString('2026-02-30')).toBeNull();
    expect(parseLocalDateString('2026-13-01')).toBeNull();
    expect(parseLocalDateString('2026-2-01')).toBeNull();
  });

  it('adds calendar days through daylight-saving transitions', () => {
    const originalTimezone = process.env.TZ;
    process.env.TZ = 'America/New_York';
    try {
      const beforeSpringForward = new Date(2026, 2, 7);
      const afterSpringForward = addCalendarDays(beforeSpringForward, 1);
      expect(toLocalDateString(afterSpringForward)).toBe('2026-03-08');
      expect(afterSpringForward.getHours()).toBe(0);

      const beforeFallBack = new Date(2026, 9, 31);
      const afterFallBack = addCalendarDays(beforeFallBack, 1);
      expect(toLocalDateString(afterFallBack)).toBe('2026-11-01');
      expect(afterFallBack.getHours()).toBe(0);
    } finally {
      if (originalTimezone === undefined) delete process.env.TZ;
      else process.env.TZ = originalTimezone;
    }
  });
});
