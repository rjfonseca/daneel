import { Component, Suspense, useEffect, useState, type ReactNode } from 'react';

interface DeferredPanelProps {
  isOpen: boolean;
  label: string;
  children: ReactNode;
}

class PanelBoundary extends Component<{ isOpen: boolean; children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    if (this.state.failed) {
      return this.props.isOpen ? (
        <div role="alert" className="fixed right-6 top-24 z-50 rounded-lg border border-rose-500/40 bg-[#0b0c13] p-4 text-sm text-slate-100">
          <p>Could not load this panel. Reload the page to try again.</p>
          <button className="mt-2 rounded-sm border border-white/20 px-3 py-1 cosmic-focus-visible" onClick={() => window.location.reload()}>Reload page</button>
        </div>
      ) : null;
    }
    return this.props.children;
  }
}

// Retain a panel after first use so closing it preserves its local state.
export function DeferredPanel({ isOpen, label, children }: DeferredPanelProps) {
  const [hasOpened, setHasOpened] = useState(isOpen);
  useEffect(() => { if (isOpen) setHasOpened(true); }, [isOpen]);
  if (!isOpen && !hasOpened) return null;
  return (
    <PanelBoundary isOpen={isOpen}>
      <Suspense fallback={isOpen ? <div role="status" className="fixed right-6 top-24 z-50 rounded-lg border border-indigo-500/30 bg-[#0b0c13] px-4 py-3 text-sm text-indigo-100">{label}</div> : null}>
        {children}
      </Suspense>
    </PanelBoundary>
  );
}
