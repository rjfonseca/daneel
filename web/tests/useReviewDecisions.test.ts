import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { useReviewDecisions } from '../src/hooks/useReviewDecisions';

const STORAGE_KEY = 'daneel:knowledge_review_decisions';

describe('useReviewDecisions', () => {
  beforeEach(() => localStorage.clear());

  it('recovers from malformed saved data', () => {
    localStorage.setItem(STORAGE_KEY, '{bad json');
    const { result } = renderHook(() => useReviewDecisions());
    expect(result.current.decisions).toEqual([]);
    act(() => result.current.save('notes/focus.md', 'notes/candidate.md'));
    expect(result.current.decisions).toEqual([{ focusId: 'notes/focus.md', candidateId: 'notes/candidate.md', decision: 'accepted' }]);
  });

  it('persists acceptance and ignore decisions by note IDs and resets a decision', () => {
    const { result, unmount } = renderHook(() => useReviewDecisions());
    act(() => result.current.save('notes/focus.md', 'notes/accepted.md'));
    act(() => result.current.ignore('notes/focus.md', 'notes/ignored.md'));
    unmount();
    const reopened = renderHook(() => useReviewDecisions());
    expect(reopened.result.current.decisions).toEqual([
      { focusId: 'notes/focus.md', candidateId: 'notes/accepted.md', decision: 'accepted' },
      { focusId: 'notes/focus.md', candidateId: 'notes/ignored.md', decision: 'ignored' },
    ]);
    act(() => reopened.result.current.reset('notes/focus.md', 'notes/accepted.md'));
    expect(reopened.result.current.decisions).toEqual([{ focusId: 'notes/focus.md', candidateId: 'notes/ignored.md', decision: 'ignored' }]);
  });

  it('keeps state usable if localStorage writes fail', () => {
    const original = Storage.prototype.setItem;
    Storage.prototype.setItem = () => { throw new Error('unavailable'); };
    const { result } = renderHook(() => useReviewDecisions());
    act(() => result.current.save('notes/focus.md', 'notes/candidate.md'));
    expect(result.current.decisions[0].decision).toBe('accepted');
    Storage.prototype.setItem = original;
  });
});
