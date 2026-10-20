import { RoutineStatusItem, UpsertRoutineResponse } from '../src/gen/api/v1/graph_pb';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { RoutineEditorModal } from '../src/components/hud/RoutineEditorModal';
import { client } from '../src/lib/apiClient';

vi.mock('../src/lib/apiClient', () => ({
  client: {
    upsertRoutine: vi.fn(),
  },
}));

describe('RoutineEditorModal Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders correctly when isOpen is true in create mode', () => {
    render(
      <RoutineEditorModal
        isOpen={true}
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initialRoutine={null}
      />
    );

    expect(screen.getByTestId('routine-editor-modal')).toBeDefined();
    expect(screen.getByText('CREATE NEW SCHEDULED ROUTINE')).toBeDefined();
    expect(screen.getByTestId('routine-input-name')).toBeDefined();
    expect(screen.getByTestId('routine-input-command')).toBeDefined();
    expect(screen.getByTestId('routine-input-schedule')).toBeDefined();
  });

  it('returns null when isOpen is false', () => {
    const { container } = render(
      <RoutineEditorModal
        isOpen={false}
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initialRoutine={null}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it('populates fields in edit mode and disables name input', () => {
    const initialRoutine = new RoutineStatusItem({
      name: 'tidy',
      title: 'Tidy Vault',
      description: 'Auto-clean vault links',
      schedule: '0 */6 * * *',
      command: 'tidy',
      enabled: true,
      category: 'deterministic',
    });

    render(
      <RoutineEditorModal
        isOpen={true}
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initialRoutine={initialRoutine}
      />
    );

    expect(screen.getByText('EDIT ROUTINE: tidy')).toBeDefined();
    const nameInput = screen.getByTestId('routine-input-name') as HTMLInputElement;
    expect(nameInput.value).toBe('tidy');
    expect(nameInput.disabled).toBe(true);

    const scheduleInput = screen.getByTestId('routine-input-schedule') as HTMLInputElement;
    expect(scheduleInput.value).toBe('0 */6 * * *');
  });

  it('submits valid data to client.upsertRoutine and calls onSaved', async () => {
    vi.mocked(client.upsertRoutine).mockResolvedValue(new UpsertRoutineResponse({ success: true }));
    const onSaved = vi.fn();
    const onClose = vi.fn();

    render(
      <RoutineEditorModal
        isOpen={true}
        onClose={onClose}
        onSaved={onSaved}
        initialRoutine={null}
      />
    );

    fireEvent.change(screen.getByTestId('routine-input-name'), {
      target: { value: 'nightly-backup' },
    });
    fireEvent.change(screen.getByTestId('routine-input-command'), {
      target: { value: 'backup --full' },
    });
    fireEvent.click(screen.getByText('Daily (06:00)'));

    fireEvent.click(screen.getByTestId('routine-save-btn'));

    await waitFor(() => {
      expect(client.upsertRoutine).toHaveBeenCalledWith({
        name: 'nightly-backup',
        command: 'backup --full',
        schedule: 'daily',
        description: '',
        enabled: true,
      });
      expect(onSaved).toHaveBeenCalledWith('nightly-backup');
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('displays error message when upsertRoutine fails', async () => {
    vi.mocked(client.upsertRoutine).mockResolvedValue(new UpsertRoutineResponse({
      success: false,
      error: 'Invalid cron expression format',
    }));

    render(
      <RoutineEditorModal
        isOpen={true}
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initialRoutine={null}
      />
    );

    fireEvent.change(screen.getByTestId('routine-input-name'), {
      target: { value: 'failing-routine' },
    });

    fireEvent.click(screen.getByTestId('routine-save-btn'));

    await waitFor(() => {
      expect(screen.getByText('Invalid cron expression format')).toBeDefined();
    });
  });
});
