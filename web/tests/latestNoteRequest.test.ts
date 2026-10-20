import { describe, expect, it, vi } from 'vitest';
import { loadLatestNote } from '../src/lib/latestNoteRequest';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(done => { resolve = done; });
  return { promise, resolve };
}

describe('loadLatestNote', () => {
  it('does not replace a newer note with an older response', async () => {
    const first = deferred<string>();
    const second = deferred<string>();
    const requestId = { current: 0 };
    const show = vi.fn();
    const loadA = loadLatestNote(requestId, () => first.promise, show, () => {});
    const loadB = loadLatestNote(requestId, () => second.promise, show, () => {});
    second.resolve('Beta');
    await loadB;
    first.resolve('Alpha');
    await loadA;
    expect(show).toHaveBeenCalledOnce();
    expect(show).toHaveBeenCalledWith('Beta');
  });

  it('ignores responses after the drawer closes', async () => {
    const pending = deferred<string>();
    const requestId = { current: 0 };
    const show = vi.fn();
    const request = loadLatestNote(requestId, () => pending.promise, show, () => {});
    requestId.current += 1;
    pending.resolve('Late');
    await request;
    expect(show).not.toHaveBeenCalled();
  });
});
