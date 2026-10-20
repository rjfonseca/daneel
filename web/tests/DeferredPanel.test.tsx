import { Suspense, lazy, useState } from 'react';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { DeferredPanel } from '../src/components/DeferredPanel';

function Counter({ isOpen }: { isOpen: boolean }) {
  const [count, setCount] = useState(0);
  return isOpen ? <button onClick={() => setCount(value => value + 1)}>Count {count}</button> : null;
}

describe('DeferredPanel', () => {
  it('loads on first open, shows loading feedback, and retains panel state after closing', async () => {
    let finishLoading!: (module: { default: typeof Counter }) => void;
    const loader = vi.fn(() => new Promise<{ default: typeof Counter }>(resolve => { finishLoading = resolve; }));
    const Panel = lazy(loader);
    const view = (isOpen: boolean) => (
      <DeferredPanel isOpen={isOpen} label="Loading tools…">
        <Panel isOpen={isOpen} />
      </DeferredPanel>
    );
    const { rerender } = render(view(false));
    expect(loader).not.toHaveBeenCalled();
    rerender(view(true));
    expect(loader).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('status')).toHaveTextContent('Loading tools…');
    await act(async () => { finishLoading({ default: Counter }); });
    fireEvent.click(screen.getByRole('button', { name: 'Count 0' }));
    expect(screen.getByRole('button', { name: 'Count 1' })).toBeVisible();
    rerender(view(false));
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    rerender(view(true));
    expect(screen.getByRole('button', { name: 'Count 1' })).toBeVisible();
    expect(loader).toHaveBeenCalledTimes(1);
  });

  it('keeps the surrounding interface visible while a panel loads', () => {
    const Panel = lazy(() => new Promise<{ default: typeof Counter }>(() => {}));
    render(<Suspense fallback="Application unavailable">
      <p>Galaxy remains visible</p>
      <DeferredPanel isOpen label="Loading tools…"><Panel isOpen /></DeferredPanel>
    </Suspense>);
    expect(screen.getByText('Galaxy remains visible')).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent('Loading tools…');
    expect(screen.queryByText('Application unavailable')).not.toBeInTheDocument();
  });
  it('keeps the galaxy usable and explains a failed panel download', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    const failure = new Error('Network unavailable');
    const handleExpectedError = (event: ErrorEvent) => {
      if (event.error === failure) event.preventDefault();
    };
    window.addEventListener('error', handleExpectedError);
    const Panel = lazy(() => Promise.reject(failure));
    try {
      render(<><p>Galaxy remains visible</p><DeferredPanel isOpen label="Loading tools…"><Panel /></DeferredPanel></>);
      expect(await screen.findByRole('alert')).toHaveTextContent('Could not load this panel');
      expect(screen.getByText('Galaxy remains visible')).toBeVisible();
      expect(screen.getByRole('button', { name: 'Reload page' })).toBeVisible();
    } finally {
      window.removeEventListener('error', handleExpectedError);
      consoleError.mockRestore();
    }
  });

});
