import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { calculateOrbitalLayout, OrbitalNodeInput } from '../src/lib/orbitalRings';
import { AgentPresenceCard } from '../src/components/hud/AgentPresenceCard';

describe('Hooks Telemetry & Orbital Integration', () => {
  it('places hooks telemetry satellite on the Seldon Perimeter', () => {
    const nodes: OrbitalNodeInput[] = [
      { id: 'sat-routine-1', ring: 'routines', x: 0, y: 0 },
      { id: 'sat-skill-1', ring: 'skills', x: 0, y: 0 },
      { id: 'sat-hooks-1', ring: 'hooks', x: 0, y: 0 },
    ];

    const layout = calculateOrbitalLayout(nodes, { defaultRadius: 6500 });
    expect(layout.rings.length).toBeGreaterThan(0);
    expect(layout.nodePositions.has('sat-hooks-1')).toBe(true);

    const pos = layout.nodePositions.get('sat-hooks-1');
    expect(pos).toBeDefined();
    expect(pos?.ring).toBe('hooks');
    expect(typeof pos?.x).toBe('number');
    expect(typeof pos?.y).toBe('number');
  });

  it('renders AgentPresenceCard with English text and active sessions', () => {
    render(
      <AgentPresenceCard
        sessions={[
          {
            sessionId: 'sess-antigravity-999',
            agent: 'antigravity',
            event: 'PostToolUse',
            cwd: '/home/user/work/daneel',
            timestamp: Date.now(),
          },
        ]}
      />
    );

    expect(screen.getByText('Agent Ambient Telemetry')).toBeDefined();
    expect(screen.getByText('1 Active')).toBeDefined();
    expect(screen.getByText('antigravity')).toBeDefined();
    expect(screen.getByText(/daneel/)).toBeDefined();
  });
});
