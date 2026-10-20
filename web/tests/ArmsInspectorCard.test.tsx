import { renderAsync } from './helpers/renderAsync';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { screen, fireEvent, cleanup, act } from '@testing-library/react';
import { ArmsInspectorCard } from '../src/components/ArmsInspectorCard';
import type { UseRoutineExecutionResult } from '../src/hooks/useRoutineExecution';

describe('ArmsInspectorCard Component', () => {
  const mockStationRoutine = {
    id: 'routine:tidy',
    title: 'Auto-Heal Links (tidy)',
    ring: 'routines',
    status: 'ONLINE',
    icon: 'clock',
  };

  const mockStationSkill = {
    id: 'skill:vault-dream-cycle',
    title: 'AI Skill: vault-dream-cycle',
    ring: 'skills',
    status: 'ONLINE',
    icon: 'sparkles',
  };

  const mockStationNonExecutable = {
    id: 'notes/random-satellite',
    title: 'Generic Satellite',
    ring: 'unknown',
    status: 'OFFLINE',
  };

  const createMockExecution = (overrides?: Partial<UseRoutineExecutionResult>): UseRoutineExecutionResult => ({
    activeExitCode: null,
    logs: [],
    isRunning: false,
    runningRoutineName: null,
    verbose: false,
    setVerbose: vi.fn(),
    executeRoutine: vi.fn(),
    abortRoutine: vi.fn(),
    clearLogs: vi.fn(),
    ...overrides,
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders station specification and details in spec tab by default', async () => {
    const onClose = vi.fn();
    await renderAsync(
      <ArmsInspectorCard
        station={mockStationRoutine}
        contentMarkdown={"# Tidy Specification\n\nDetailed docs here."}
        onClose={onClose}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
      />
    );

    expect(screen.getByTestId('arms-inspector-card')).toBeDefined();
    expect(screen.getByText('Auto-Heal Links (tidy)')).toBeDefined();
    expect(screen.getByText('Tidy Specification')).toBeDefined();
    expect(screen.getByText('Detailed docs here.')).toBeDefined();
    expect(screen.getByTestId('arms-tab-spec')).toBeDefined();
    expect(screen.getByTestId('arms-tab-terminal')).toBeDefined();
    expect(screen.getByTestId('arms-execute-routine-btn')).toBeDefined();
  });

  it('calls onClose when close button is clicked', async () => {
    const onClose = vi.fn();
    await renderAsync(
      <ArmsInspectorCard
        station={mockStationRoutine}
        contentMarkdown="# Docs"
        onClose={onClose}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
      />
    );

    const closeBtn = screen.getByTestId('close-arms-inspector');
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalled();
  });

  it('switches between Spec and Terminal tabs', async () => {
    const execution = createMockExecution({
      logs: ['[Info] Seldon Core online'],
    });

    await renderAsync(
      <ArmsInspectorCard
        station={mockStationRoutine}
        contentMarkdown="# Specs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        execution={execution}
      />
    );

    // Initial state: Spec tab
    expect(screen.getByText('Specs')).toBeDefined();
    expect(screen.queryByTestId('arms-terminal')).toBeNull();

    // Switch to Terminal tab
    const terminalTab = screen.getByTestId('arms-tab-terminal');
    fireEvent.click(terminalTab);

    expect(screen.getByTestId('arms-terminal')).toBeDefined();
    expect(screen.getByText('[Info] Seldon Core online')).toBeDefined();

    // Switch back to Spec tab
    const specTab = screen.getByTestId('arms-tab-spec');
    await act(async () => {
      fireEvent.click(specTab);
    });

    expect(screen.getByText('Specs')).toBeDefined();
    expect(screen.queryByTestId('arms-terminal')).toBeNull();
  });

  it('triggers executeRoutine and switches to terminal when Run Routine is clicked', async () => {
    const executeRoutine = vi.fn();
    const execution = createMockExecution({
      executeRoutine,
      verbose: true,
    });

    await renderAsync(
      <ArmsInspectorCard
        station={mockStationSkill}
        contentMarkdown="# AI Skill Specs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        execution={execution}
      />
    );

    const runBtn = screen.getByTestId('arms-execute-routine-btn');
    fireEvent.click(runBtn);

    expect(executeRoutine).toHaveBeenCalledWith('vault-dream-cycle', true);
    // Should now be on terminal tab
    expect(screen.getByTestId('arms-terminal')).toBeDefined();
  });

  it('triggers harvest:jira when Run Routine is clicked on Jira Harvester', async () => {
    const executeRoutine = vi.fn();
    const execution = createMockExecution({
      executeRoutine,
      verbose: false,
    });

    await renderAsync(
      <ArmsInspectorCard
        station={{
          id: 'harvester:jira',
          title: 'Jira Harvester',
          ring: 'harvesters',
          status: 'ONLINE',
          icon: 'jira',
        }}
        contentMarkdown="# Jira Specs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        execution={execution}
      />
    );

    const runBtn = screen.getByTestId('arms-execute-routine-btn');
    fireEvent.click(runBtn);

    expect(executeRoutine).toHaveBeenCalledWith('harvest:jira', false);
    expect(screen.getByTestId('arms-terminal')).toBeDefined();
  });

  it('calls onOpenCockpit with resolved routine name when cockpit button is clicked', async () => {
    const onOpenCockpit = vi.fn();

    await renderAsync(
      <ArmsInspectorCard
        station={mockStationRoutine}
        contentMarkdown="# Specs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        onOpenCockpit={onOpenCockpit}
      />
    );

    const cockpitBtn = screen.getByTestId('arms-open-cockpit-btn');
    expect(cockpitBtn).toBeDefined();

    fireEvent.click(cockpitBtn);
    expect(onOpenCockpit).toHaveBeenCalledWith('tidy');
  });

  it('renders constellation reset button and triggers callback when active', async () => {
    const onResetConstellation = vi.fn();

    await renderAsync(
      <ArmsInspectorCard
        station={mockStationRoutine}
        contentMarkdown="# Specs"
        onClose={vi.fn()}
        isConstellationActive={true}
        onResetConstellation={onResetConstellation}
      />
    );

    const resetBtn = screen.getByTestId('constellation-reset-btn');
    expect(resetBtn).toBeDefined();

    fireEvent.click(resetBtn);
    expect(onResetConstellation).toHaveBeenCalled();
  });

  it('does not render execution tabs if station is not executable', async () => {
    await renderAsync(
      <ArmsInspectorCard
        station={mockStationNonExecutable}
        contentMarkdown="# Generic Specs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
      />
    );

    expect(screen.queryByTestId('arms-tab-spec')).toBeNull();
    expect(screen.queryByTestId('arms-tab-terminal')).toBeNull();
    expect(screen.queryByTestId('arms-execute-routine-btn')).toBeNull();
    expect(screen.getByText('Generic Specs')).toBeDefined();
  });

  it('renders SkillParameterDeck for AI skill satellite and dispatches agent with parameters', async () => {
    const executeRoutine = vi.fn();
    const execution = createMockExecution({
      executeRoutine,
    });

    await renderAsync(
      <ArmsInspectorCard
        station={{
          id: 'skill:meeting-preflight',
          title: 'Meeting Preflight',
          ring: 'skills',
          status: 'ONLINE',
          icon: 'sparkles',
        }}
        contentMarkdown="# Preflight Docs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        execution={execution}
      />
    );

    // Skill parameter deck is rendered
    expect(screen.getByTestId('arms-skill-deck')).toBeDefined();
    expect(screen.getByTestId('arms-skill-input-attendee')).toBeDefined();
    expect(screen.getByTestId('arms-skill-input-topic')).toBeDefined();

    // Fill parameters
    fireEvent.change(screen.getByTestId('arms-skill-input-attendee'), {
      target: { value: 'VP Engineering' },
    });
    fireEvent.change(screen.getByTestId('arms-skill-input-topic'), {
      target: { value: 'Staff Calibration' },
    });

    // Dispatch agent
    const dispatchBtn = screen.getByTestId('arms-skill-dispatch-btn');
    fireEvent.click(dispatchBtn);

    expect(executeRoutine).toHaveBeenCalledWith(
      'meeting-preflight',
      false,
      expect.objectContaining({
        attendee: 'VP Engineering',
        topic: 'Staff Calibration',
      })
    );
    // Switched to terminal tab
    expect(screen.getByTestId('arms-terminal')).toBeDefined();
  });

  it('renders Harvester deck with Sync Platform and Sync All actions', async () => {
    const executeRoutine = vi.fn();
    const execution = createMockExecution({
      executeRoutine,
    });

    await renderAsync(
      <ArmsInspectorCard
        station={{
          id: 'harvester:github',
          title: 'GitHub Harvester',
          ring: 'harvesters',
          status: 'ONLINE',
          icon: 'github',
        }}
        contentMarkdown="# GitHub Docs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        execution={execution}
      />
    );

    expect(screen.getByTestId('arms-app-deck')).toBeDefined();
    const syncPlatformBtn = screen.getByTestId('arms-sync-platform-btn');

    fireEvent.click(syncPlatformBtn);
    expect(executeRoutine).toHaveBeenCalledWith('harvest:github', false);

    // Switch back to spec tab to test Sync All
    fireEvent.click(screen.getByTestId('arms-tab-spec'));
    const syncAllBtn = screen.getByTestId('arms-sync-all-btn');
    fireEvent.click(syncAllBtn);
    expect(executeRoutine).toHaveBeenCalledWith('harvest', false);
  });

  it('renders Routine deck with Cadence and Run Routine action', async () => {
    const executeRoutine = vi.fn();
    const execution = createMockExecution({
      executeRoutine,
    });

    await renderAsync(
      <ArmsInspectorCard
        station={mockStationRoutine}
        contentMarkdown="# Routine Docs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        execution={execution}
      />
    );

    expect(screen.getByTestId('arms-routine-deck')).toBeDefined();
    const runBtn = screen.getByTestId('arms-run-routine-deck-btn');

    fireEvent.click(runBtn);
    expect(executeRoutine).toHaveBeenCalledWith('tidy', false);
  });

  it('copies terminal output to clipboard when copy logs button is clicked', async () => {
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    });

    const execution = createMockExecution({
      logs: ['[Info] Seldon Core online', '✅ Synaptic graph indexed'],
    });

    await renderAsync(
      <ArmsInspectorCard
        station={mockStationRoutine}
        contentMarkdown="# Specs"
        onClose={vi.fn()}
        isConstellationActive={false}
        onResetConstellation={vi.fn()}
        execution={execution}
      />
    );

    // Switch to Terminal
    fireEvent.click(screen.getByTestId('arms-tab-terminal'));

    const copyLogsBtn = screen.getByTestId('arms-copy-logs-btn');
    await act(async () => {
      fireEvent.click(copyLogsBtn);
    });

    expect(writeTextMock).toHaveBeenCalledWith(
      "[Info] Seldon Core online\n✅ Synaptic graph indexed"
    );
  });
});
