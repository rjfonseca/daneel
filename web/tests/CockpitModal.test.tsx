import { GetRoutinesOverviewResponse, GetStaffTelemetryResponse, GetCognitiveTelemetryResponse, GetRoutineHistoryResponse, RoutineOutputEvent, DeleteRoutineResponse } from '../src/gen/api/v1/graph_pb';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react';
import { CockpitModal } from '../src/components/hud/CockpitModal';
import { client } from '../src/lib/apiClient';

vi.mock('../src/lib/apiClient', () => ({
  client: {
    getRoutinesOverview: vi.fn(),
    getStaffTelemetry: vi.fn(),
    getCognitiveTelemetry: vi.fn(),
    executeRoutine: vi.fn(),
    upsertRoutine: vi.fn(),
    deleteRoutine: vi.fn(),
    getRoutineHistory: vi.fn(),
  },
}));

describe('CockpitModal Component', () => {
  const mockRoutines = [
    {
      name: 'tidy',
      title: 'Auto-Heal Links (tidy)',
      description: 'Converts [[WikiLinks]], repairs dangling links, synchronizes MOCs',
      category: 'deterministic',
      status: 'ready',
      schedule: 'hourly',
      lastRunTimestamp: BigInt(1726000000),
      lastResult: 'success',
      nextTriggerTimestamp: BigInt(1792000000),
    },
    {
      name: 'doctor',
      title: 'Vault Diagnostics (doctor)',
      description: 'Validates OKF frontmatter, checks dangling references',
      category: 'deterministic',
      status: 'ready',
      lastRunTimestamp: BigInt(1726000000),
    },
    {
      name: 'vault-dream-cycle',
      title: 'AI Skill: vault-dream-cycle',
      description: 'Autonomous cognitive consolidation for AI agents',
      category: 'ai_skill',
      status: 'ready',
      lastRunTimestamp: BigInt(1726000000),
    },
  ];

  const mockTelemetry = {
    strategicAHours: 4.5,
    localImpactBHours: 2.0,
    glueWorkCHours: 1.5,
    lowImpactDHours: 0.5,
    overheadEHours: 0.5,
    totalHours: 9.0,
    leverageRate: 0.667,
    topInitiatives: ['AI Platform', 'Core Engine'],
    rollingWeeks: [
      {
        periodKey: '2026-W39',
        label: 'Week 39 (Sep 21 - Sep 27)',
        daysCount: 5,
        strategicAHours: 4.5,
        localImpactBHours: 2.0,
        glueWorkCHours: 1.5,
        lowImpactDHours: 0.5,
        overheadEHours: 0.5,
        totalHours: 9.0,
        leverageRate: 0.667,
        diagnosis: '🚀 Strategic Multiplication',
      },
    ],
    monthlyHistory: [
      {
        periodKey: '2026-09',
        label: 'September 2026',
        daysCount: 20,
        strategicAHours: 35.0,
        localImpactBHours: 15.0,
        glueWorkCHours: 18.0,
        lowImpactDHours: 1.8,
        overheadEHours: 8.0,
        totalHours: 77.8,
        leverageRate: 0.681,
        diagnosis: '🚀 Strategic Multiplication',
      },
    ],
  };

  const mockCognitiveTelemetry = {
    lastCycleDurationMs: BigInt(2500),
    stageTimings: [
      {
        name: 'Zettelkasten Linking',
        durationMs: BigInt(450),
        itemsCount: 12,
        description: 'Cross-link discovery across domains',
      },
      {
        name: 'MOC Topology Refresh',
        durationMs: BigInt(120),
        itemsCount: 4,
        description: 'Map of content recalculation',
      },
    ],
    bottlenecks: ['Stage [Zettelkasten Linking] took 450ms'],
    inducedSyntheses: ['Synthesized concept: Autonomous Memory Consolidation'],
    evolutionLogPath: 'docs/reports/evolution_ledger.md',
    scannedTurns: 42,
    missedOpportunitiesCount: 3,
    knowledgeGapsCount: 1,
    recentEvals: [
      {
        agent: 'antigravity',
        topic: 'Missing ADR check',
        classification: 'missed_opportunity',
        description: 'Agent skipped checking docs/adr before implementing protocol',
        suggestedAction: 'Consult AGENTS.md invariant 2.1 before architectural changes',
        timestamp: BigInt(1726000000),
      },
    ],
    totalFilesScanned: 156,
    brokenLinksCount: 0,
    lastDreamTimestamp: BigInt(1726000000),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(client.getRoutinesOverview).mockResolvedValue(new GetRoutinesOverviewResponse({ routines: mockRoutines }));
    vi.mocked(client.getStaffTelemetry).mockResolvedValue(new GetStaffTelemetryResponse(mockTelemetry));
    vi.mocked(client.getCognitiveTelemetry).mockResolvedValue(new GetCognitiveTelemetryResponse(mockCognitiveTelemetry));
    vi.mocked(client.getRoutineHistory).mockResolvedValue(new GetRoutineHistoryResponse({ runs: [] }));
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders correctly when isOpen is true', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    expect(screen.getByTestId('cockpit-modal')).toBeDefined();
    expect(screen.getByText('Routines and activity')).toBeDefined();

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
      expect(screen.getByText('Vault Diagnostics (doctor)')).toBeDefined();
      expect(screen.getByText('AI Skill: vault-dream-cycle')).toBeDefined();
    });
  });

  it('returns null when isOpen is false', () => {
    const { container } = render(<CockpitModal isOpen={false} onClose={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('calls onClose when close button is clicked', async () => {
    const onClose = vi.fn();
    render(<CockpitModal isOpen={true} onClose={onClose} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const closeBtn = screen.getByTestId('cockpit-close-btn');
    fireEvent.click(closeBtn);

    expect(onClose).toHaveBeenCalled();
  });

  it('calls onClose when Escape key is pressed', async () => {
    const onClose = vi.fn();
    render(<CockpitModal isOpen={true} onClose={onClose} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    fireEvent.keyDown(window, { key: 'Escape' });

    expect(onClose).toHaveBeenCalled();
  });

  it('filters routines by category', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    // Click "AI Skills" filter
    const aiSkillsBtn = screen.getByText('AI Skills');
    fireEvent.click(aiSkillsBtn);

    expect(screen.queryByTestId('cockpit-routine-tidy')).toBeNull();
    expect(screen.getByText('AI Skill: vault-dream-cycle')).toBeDefined();

    // Click "Routines" filter
    const routinesBtn = screen.getByText('Routines');
    fireEvent.click(routinesBtn);

    expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    expect(screen.queryByText('AI Skill: vault-dream-cycle')).toBeNull();
  });

  it('filters routines by text search query', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const searchInput = screen.getByPlaceholderText('Filter routines or skills...');
    fireEvent.change(searchInput, { target: { value: 'doctor' } });

    expect(screen.getByText('Vault Diagnostics (doctor)')).toBeDefined();
    expect(screen.queryByTestId('cockpit-routine-tidy')).toBeNull();
    expect(screen.queryByText('AI Skill: vault-dream-cycle')).toBeNull();
  });

  it('shows a plain-language routine preview before execution', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByTestId('cockpit-routine-tidy')).toBeInTheDocument());

    fireEvent.click(screen.getByTestId('cockpit-routine-tidy'));

    expect(screen.getByTestId('cockpit-routine-preview')).toHaveTextContent('Every hour');
    expect(screen.getByTestId('cockpit-routine-preview')).toHaveTextContent('Completed successfully');
    expect(screen.getByTestId('cockpit-routine-preview')).toHaveTextContent('Converts [[WikiLinks]], repairs dangling links, synchronizes MOCs');
    expect(client.executeRoutine).not.toHaveBeenCalled();
  });

  it('switches to Staff Telemetry tab and displays hours and initiatives', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-tab-telemetry')).toBeDefined();
    });

    const telemetryTab = screen.getByTestId('cockpit-tab-telemetry');
    fireEvent.click(telemetryTab);

    await waitFor(() => {
      expect(screen.getByText('[A] Strategic Impact')).toBeDefined();
      expect(screen.getByText('4.5h')).toBeDefined();
      expect(screen.getByText('2.0h')).toBeDefined();
      expect(screen.getByText('[D] Low Impact')).toBeDefined();
      expect(screen.getByText('66.7%')).toBeDefined();
      expect(screen.getByText('HIGH LEVERAGE (STRATEGIC)')).toBeDefined();
      expect(screen.getByText('Rolling Weekly Comparative Matrix')).toBeDefined();
      expect(screen.getByTestId('cockpit-weekly-2026-W39')).toBeDefined();
      expect(screen.getByText('Historical Monthly Telemetry')).toBeDefined();
      expect(screen.getByText('AI Platform')).toBeDefined();
      expect(screen.getByText('Core Engine')).toBeDefined();
    });
  });

  it('executes a routine and displays streaming output in terminal', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: '🚀 Starting tidy execution...', isCompleted: false, exitCode: 0, isError: false });
      yield new RoutineOutputEvent({ line: '✅ Processed 10 notes.', isCompleted: true, exitCode: 0, isError: false });
    }

    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const primaryRunBtn = screen.getByTestId('cockpit-primary-run-btn');
    await act(async () => {
      fireEvent.click(primaryRunBtn);
    });

    await waitFor(() => {
      expect(screen.getByText('🚀 Starting tidy execution...')).toBeDefined();
      expect(screen.getByText('✅ Processed 10 notes.')).toBeDefined();
    });

    expect(client.executeRoutine).toHaveBeenCalledWith(
      expect.objectContaining({
        routineName: 'tidy',
        verbose: false,
      }),
      expect.any(Object)
    );
  });

  it('toggles verbose mode and passes verbose flag to execution', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: '✨ Done', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const verboseBtn = screen.getByTestId('cockpit-verbose-toggle');
    fireEvent.click(verboseBtn);

    const primaryRunBtn = screen.getByTestId('cockpit-primary-run-btn');
    await act(async () => {
      fireEvent.click(primaryRunBtn);
    });

    expect(client.executeRoutine).toHaveBeenCalledWith(
      expect.objectContaining({
        verbose: true,
      }),
      expect.any(Object)
    );
  });

  it('clears terminal logs when clear button is clicked', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'Log line to clear', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const primaryRunBtn = screen.getByTestId('cockpit-primary-run-btn');
    await act(async () => {
      fireEvent.click(primaryRunBtn);
    });

    await waitFor(() => {
      expect(screen.getByText('Log line to clear')).toBeDefined();
    });

    const clearBtn = screen.getByTestId('cockpit-clear-btn');
    fireEvent.click(clearBtn);

    expect(screen.queryByText('Log line to clear')).toBeNull();
  });

  it('renders SkillParameterDeck and dispatches AI skill with parameters', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: '⚡ [Cockpit] Initializing AI Skill...', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('AI Skill: vault-dream-cycle')).toBeDefined();
    });

    // Select the AI Skill card
    const skillCard = screen.getByTestId('cockpit-routine-vault-dream-cycle');
    fireEvent.click(skillCard);

    // Skill parameter deck should now be visible
    expect(screen.getByTestId('cockpit-skill-deck')).toBeDefined();
    expect(screen.getByTestId('cockpit-skill-input-date')).toBeDefined();

    // Click dispatch agent in the deck
    const dispatchBtn = screen.getByTestId('cockpit-skill-dispatch-btn');
    await act(async () => {
      fireEvent.click(dispatchBtn);
    });

    expect(client.executeRoutine).toHaveBeenCalledWith(
      expect.objectContaining({
        routineName: 'vault-dream-cycle',
        params: expect.any(Object),
      }),
      expect.any(Object)
    );

    await waitFor(() => {
      expect(screen.getByText('⚡ [Cockpit] Initializing AI Skill...')).toBeDefined();
    });
  });

  it('copies terminal output in CockpitModal', async () => {
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    });

    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'System telemetry online', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const primaryRunBtn = screen.getByTestId('cockpit-primary-run-btn');
    await act(async () => {
      fireEvent.click(primaryRunBtn);
    });

    await waitFor(() => {
      expect(screen.getByText('System telemetry online')).toBeDefined();
    });

    const copyLogsBtn = screen.getByTestId('cockpit-copy-logs-btn');
    await act(async () => {
      fireEvent.click(copyLogsBtn);
    });

    expect(writeTextMock).toHaveBeenCalledWith(
      expect.stringContaining('System telemetry online')
    );
  });

  it('renders schedule badge when a routine has a schedule', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-schedule-tidy')).toBeDefined();
      expect(screen.getByTestId('cockpit-routine-schedule-tidy').textContent).toContain('Every hour');
    });
  });

  it('opens RoutineEditorModal in create mode when clicking New Routine button', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-add-routine-btn')).toBeDefined();
    });

    const addBtn = screen.getByTestId('cockpit-add-routine-btn');
    fireEvent.click(addBtn);

    expect(screen.getByTestId('routine-editor-modal')).toBeDefined();
    expect(screen.getByText('CREATE NEW SCHEDULED ROUTINE')).toBeDefined();
  });

  it('opens RoutineEditorModal in edit mode when clicking Edit button on a routine card', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-edit-btn-tidy')).toBeDefined();
    });

    const editBtn = screen.getByTestId('cockpit-edit-btn-tidy');
    fireEvent.click(editBtn);

    expect(screen.getByTestId('routine-editor-modal')).toBeDefined();
    expect(screen.getByText('EDIT ROUTINE: tidy')).toBeDefined();
  });

  it('deletes a routine and refreshes overview when clicking Delete button and confirming', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    vi.mocked(client.deleteRoutine).mockResolvedValue(new DeleteRoutineResponse({ success: true }));

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-delete-btn-tidy')).toBeDefined();
    });

    const deleteBtn = screen.getByTestId('cockpit-delete-btn-tidy');
    await act(async () => {
      fireEvent.click(deleteBtn);
    });

    expect(confirmSpy).toHaveBeenCalledWith(
      expect.stringContaining('tidy')
    );
    expect(client.deleteRoutine).toHaveBeenCalledWith({ name: 'tidy' });
    // Verify getRoutinesOverview was called again to refresh list
    expect(client.getRoutinesOverview).toHaveBeenCalledTimes(2);

    confirmSpy.mockRestore();
  });

  it('switches to Agent & Metacognition tab and displays introspective metrics and evaluations', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    const metaTab = screen.getByTestId('cockpit-tab-metacognition');
    fireEvent.click(metaTab);

    await waitFor(() => {
      expect(screen.getByText('Dream Duration')).toBeDefined();
      expect(screen.getByText('2.50s')).toBeDefined();
      expect(screen.getByText('Zettelkasten Linking')).toBeDefined();
      expect(screen.getByText('450ms')).toBeDefined();
      expect(screen.getByText('Stage [Zettelkasten Linking] took 450ms')).toBeDefined();
      expect(screen.getByText('Synthesized concept: Autonomous Memory Consolidation')).toBeDefined();
      expect(screen.getByText('MISSED OPPORTUNITY')).toBeDefined();
      expect(screen.getByText('antigravity')).toBeDefined();
      expect(screen.getByText('Missing ADR check')).toBeDefined();
      expect(screen.getByText('docs/reports/evolution_ledger.md')).toBeDefined();
    });
  });

  it('dispatches dream routine with stats via Quick-Action Preset', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'Starting dream cycle...', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-quick-dream')).toBeDefined();
    });

    const quickDreamBtn = screen.getByTestId('cockpit-quick-dream');
    await act(async () => {
      fireEvent.click(quickDreamBtn);
    });

    expect(client.executeRoutine).toHaveBeenCalledWith(
      expect.objectContaining({
        routineName: 'dream',
        verbose: false,
        params: { stats: 'true' },
      }),
      expect.any(Object)
    );
  });

  it('dispatches audit_agents routine via Quick-Action Preset', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'Auditing agent transcripts...', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-quick-audit_agents')).toBeDefined();
    });

    const quickAuditBtn = screen.getByTestId('cockpit-quick-audit_agents');
    await act(async () => {
      fireEvent.click(quickAuditBtn);
    });

    expect(client.executeRoutine).toHaveBeenCalledWith(
      expect.objectContaining({
        routineName: 'audit_agents',
        verbose: false,
      }),
      expect.any(Object)
    );
  });

  it('executes selected routine when Ctrl+Enter is pressed', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'Executing tidy...', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    await act(async () => {
      fireEvent.keyDown(window, { key: 'Enter', ctrlKey: true });
    });

    expect(client.executeRoutine).toHaveBeenCalledWith(
      expect.objectContaining({
        routineName: 'tidy',
      }),
      expect.any(Object)
    );
  });

  it('refreshes all telemetry and routines when refresh button is clicked', async () => {
    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-refresh-btn')).toBeDefined();
    });

    const refreshBtn = screen.getByTestId('cockpit-refresh-btn');
    await act(async () => {
      fireEvent.click(refreshBtn);
    });

    expect(client.getRoutinesOverview).toHaveBeenCalledTimes(2);
    expect(client.getStaffTelemetry).toHaveBeenCalledTimes(2);
    expect(client.getCognitiveTelemetry).toHaveBeenCalledTimes(2);
  });

  it('searches and filters terminal logs with match count', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'Alpha log message', isCompleted: false, exitCode: 0, isError: false });
      yield new RoutineOutputEvent({ line: 'Beta message alert', isCompleted: false, exitCode: 0, isError: false });
      yield new RoutineOutputEvent({ line: 'Alpha confirmation', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const primaryRunBtn = screen.getByTestId('cockpit-primary-run-btn');
    await act(async () => {
      fireEvent.click(primaryRunBtn);
    });

    await waitFor(() => {
      expect(screen.getByText('Alpha log message')).toBeDefined();
      expect(screen.getByText('Beta message alert')).toBeDefined();
      expect(screen.getByText('Alpha confirmation')).toBeDefined();
    });

    // Toggle search bar
    const searchToggle = screen.getByTestId('cockpit-search-toggle-btn');
    await act(async () => {
      fireEvent.click(searchToggle);
    });

    const searchInput = screen.getByTestId('cockpit-search-input');
    await act(async () => {
      fireEvent.change(searchInput, { target: { value: 'Alpha' } });
    });

    // Expect 2 matches badge
    expect(screen.getByTestId('cockpit-match-count').textContent).toMatch(/2 \/ \d+ matched/);
    expect(screen.getByText('Alpha log message')).toBeDefined();
    expect(screen.getByText('Alpha confirmation')).toBeDefined();
    expect(screen.queryByText('Beta message alert')).toBeNull();
  });

  it('downloads terminal logs when download button is clicked', async () => {
    const downloadClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      expect(this.href).toBe('blob:mock-url');
      expect(this.download).toMatch(/^daneel-terminal-.*\.txt$/);
    });
    const createObjectURLMock = vi.fn().mockReturnValue('blob:mock-url');
    const revokeObjectURLMock = vi.fn();
    window.URL.createObjectURL = createObjectURLMock;
    window.URL.revokeObjectURL = revokeObjectURLMock;

    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'Log line to download', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    const primaryRunBtn = screen.getByTestId('cockpit-primary-run-btn');
    await act(async () => {
      fireEvent.click(primaryRunBtn);
    });

    await waitFor(() => {
      expect(screen.getByText('Log line to download')).toBeDefined();
    });

    const downloadBtn = screen.getByTestId('cockpit-download-logs-btn');
    fireEvent.click(downloadBtn);

    expect(createObjectURLMock).toHaveBeenCalled();
    expect(revokeObjectURLMock).toHaveBeenCalled();
    expect(downloadClick).toHaveBeenCalledOnce();
  });

  it('automatically reloads cognitive telemetry when audit_agents routine completes successfully', async () => {
    async function* mockStream() {
      yield new RoutineOutputEvent({ line: 'Auditing agents...', isCompleted: false, exitCode: 0, isError: false });
      yield new RoutineOutputEvent({ line: 'Audit finished.', isCompleted: true, exitCode: 0, isError: false });
    }
    vi.mocked(client.executeRoutine).mockReturnValue(mockStream());

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    await waitFor(() => {
      expect(client.getCognitiveTelemetry).toHaveBeenCalledTimes(1);
    });

    // Switch to agent & metacognition tab and click Run Transcript Audit
    const agentTabBtn = screen.getByTestId('cockpit-tab-metacognition');
    fireEvent.click(agentTabBtn);

    const auditBtn = screen.getByTestId('cockpit-audit-now-btn');
    await act(async () => {
      fireEvent.click(auditBtn);
    });

    await waitFor(() => {
      expect(client.getCognitiveTelemetry).toHaveBeenCalledTimes(2);
    });
  });

  it('shows Recent Runs deck when view history button is clicked', async () => {
    const mockRuns = [
      {
        id: 'run-1',
        routineName: 'tidy',
        startedAt: BigInt(1726000000),
        completedAt: BigInt(1726000010),
        durationMs: BigInt(10000),
        triggerSource: 'cli',
        exitCode: 0,
        status: 'success',
        summary: 'Repaired 3 links',
        errorMessage: '',
      },
    ];
    vi.mocked(client.getRoutineHistory).mockResolvedValue(new GetRoutineHistoryResponse({ runs: mockRuns }));

    render(<CockpitModal isOpen={true} onClose={vi.fn()} />);

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByTestId('cockpit-routine-tidy')).toBeDefined();
    });

    // Select a routine to reveal right column
    const tidyCard = screen.getByTestId('cockpit-routine-tidy');
    fireEvent.click(tidyCard);

    // Find and click the "Recent Runs" toggle
    await waitFor(() => {
      expect(screen.getByTestId('cockpit-view-history-btn')).toBeDefined();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId('cockpit-view-history-btn'));
    });

    // History deck should appear with run data
    await waitFor(() => {
      expect(screen.getByTestId('cockpit-history-container')).toBeDefined();
    });

    expect(client.getRoutineHistory).toHaveBeenCalled();
  });
});
