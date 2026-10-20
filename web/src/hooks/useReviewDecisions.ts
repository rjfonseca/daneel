import { useCallback, useState } from 'react';
import { isSafeNoteId } from '../lib/navigationHelpers';

const STORAGE_KEY = 'daneel:knowledge_review_decisions';
const MAX_DECISIONS = 200;

export type ReviewDecisionStatus = 'accepted' | 'ignored';
export interface ReviewDecision {
  focusId: string;
  candidateId: string;
  decision: ReviewDecisionStatus;
}

function normalizeDecisions(value: unknown): ReviewDecision[] {
  if (!Array.isArray(value)) return [];
  const byPair = new Map<string, ReviewDecision>();
  for (const item of value) {
    if (!item || typeof item !== 'object') continue;
    const record = item as Record<string, unknown>;
    if (!isSafeNoteId(record.focusId) || !isSafeNoteId(record.candidateId) ||
      (record.decision !== 'accepted' && record.decision !== 'ignored')) continue;
    const decision = { focusId: record.focusId, candidateId: record.candidateId, decision: record.decision } as ReviewDecision;
    byPair.set(`${decision.focusId}\u0000${decision.candidateId}`, decision);
  }
  return [...byPair.values()].slice(-MAX_DECISIONS);
}

function readDecisions(): ReviewDecision[] {
  try {
    return normalizeDecisions(JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]'));
  } catch {
    return [];
  }
}

function writeDecisions(decisions: ReviewDecision[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(decisions.slice(-MAX_DECISIONS)));
  } catch {
    // Review remains available in memory when browser storage is unavailable.
  }
}

export interface UseReviewDecisionsResult {
  decisions: ReviewDecision[];
  save: (focusId: string, candidateId: string) => void;
  ignore: (focusId: string, candidateId: string) => void;
  reset: (focusId: string, candidateId: string) => void;
}

export function useReviewDecisions(): UseReviewDecisionsResult {
  const [decisions, setDecisions] = useState(readDecisions);

  const setDecision = useCallback((focusId: string, candidateId: string, decision: ReviewDecisionStatus) => {
    if (!isSafeNoteId(focusId) || !isSafeNoteId(candidateId)) return;
    const next = normalizeDecisions([
      ...decisions,
      { focusId, candidateId, decision },
    ]);
    setDecisions(next);
    writeDecisions(next);
  }, [decisions]);

  const save = useCallback((focusId: string, candidateId: string) => {
    setDecision(focusId, candidateId, 'accepted');
  }, [setDecision]);
  const ignore = useCallback((focusId: string, candidateId: string) => {
    setDecision(focusId, candidateId, 'ignored');
  }, [setDecision]);
  const reset = useCallback((focusId: string, candidateId: string) => {
    const next = decisions.filter(item => item.focusId !== focusId || item.candidateId !== candidateId);
    setDecisions(next);
    writeDecisions(next);
  }, [decisions]);

  return { decisions, save, ignore, reset };
}
