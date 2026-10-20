import { describe, expect, it } from 'vitest';
import { resolveHeaderLayout } from '../src/lib/headerLayout';

describe('resolveHeaderLayout', () => {
  it.each([
    [320, 'compact'],
    [767, 'compact'],
    [768, 'condensed'],
    [1279, 'condensed'],
    [1280, 'full'],
    [1440, 'full'],
  ] as const)('uses the expected action layout at %i pixels', (width, expected) => {
    expect(resolveHeaderLayout(width)).toBe(expected);
  });
});
