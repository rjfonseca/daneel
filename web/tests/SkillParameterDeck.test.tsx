import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react';
import { SkillParameterDeck } from '../src/components/hud/SkillParameterDeck';

describe('SkillParameterDeck Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders dynamic parameter fields for meeting-preflight skill', () => {
    render(
      <SkillParameterDeck
        skillName="meeting-preflight"
        skillTitle="Meeting Preflight Briefing"
        onDispatch={vi.fn()}
      />
    );

    expect(screen.getByTestId('skill-param-deck')).toBeDefined();
    expect(screen.getByText('Meeting Preflight Briefing')).toBeDefined();
    expect(screen.getByTestId('skill-param-input-attendee')).toBeDefined();
    expect(screen.getByTestId('skill-param-input-topic')).toBeDefined();
    expect(screen.getByTestId('skill-param-input-date')).toBeDefined();
    expect(screen.getByTestId('skill-param-input-context')).toBeDefined();
  });

  it('updates input values when typing', () => {
    render(
      <SkillParameterDeck
        skillName="meeting-preflight"
        skillTitle="Meeting Preflight"
        onDispatch={vi.fn()}
      />
    );

    const attendeeInput = screen.getByTestId('skill-param-input-attendee') as HTMLInputElement;
    fireEvent.change(attendeeInput, { target: { value: 'Alice & Bob' } });
    expect(attendeeInput.value).toBe('Alice & Bob');

    const topicInput = screen.getByTestId('skill-param-input-topic') as HTMLInputElement;
    fireEvent.change(topicInput, { target: { value: 'Distributed Systems Architecture' } });
    expect(topicInput.value).toBe('Distributed Systems Architecture');
  });

  it('calls onDispatch with hydrated parameters when Dispatch Agent is clicked', () => {
    const onDispatch = vi.fn();
    render(
      <SkillParameterDeck
        skillName="meeting-preflight"
        skillTitle="Meeting Preflight"
        onDispatch={onDispatch}
      />
    );

    fireEvent.change(screen.getByTestId('skill-param-input-attendee'), {
      target: { value: 'Carol Vance' },
    });
    fireEvent.change(screen.getByTestId('skill-param-input-topic'), {
      target: { value: 'Security Architecture Review' },
    });

    const dispatchBtn = screen.getByTestId('skill-param-dispatch-btn');
    fireEvent.click(dispatchBtn);

    expect(onDispatch).toHaveBeenCalledWith(
      expect.objectContaining({
        attendee: 'Carol Vance',
        topic: 'Security Architecture Review',
      })
    );
  });

  it('toggles prompt protocol preview', () => {
    render(
      <SkillParameterDeck
        skillName="meeting-preflight"
        skillTitle="Meeting Preflight"
        onDispatch={vi.fn()}
      />
    );

    expect(screen.queryByTestId('skill-param-prompt-preview')).toBeNull();

    const toggleBtn = screen.getByTestId('skill-param-toggle-preview-btn');
    fireEvent.click(toggleBtn);

    expect(screen.getByTestId('skill-param-prompt-preview')).toBeDefined();
    expect(screen.getByText(/Execute meeting preflight briefing protocol/)).toBeDefined();

    // Toggle back off
    fireEvent.click(toggleBtn);
    expect(screen.queryByTestId('skill-param-prompt-preview')).toBeNull();
  });

  it('copies hydrated prompt to clipboard', async () => {
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    });

    render(
      <SkillParameterDeck
        skillName="meeting-preflight"
        skillTitle="Meeting Preflight"
        onDispatch={vi.fn()}
      />
    );

    fireEvent.change(screen.getByTestId('skill-param-input-attendee'), {
      target: { value: 'David' },
    });

    const copyBtn = screen.getByTestId('skill-param-copy-prompt-btn');
    await act(async () => {
      fireEvent.click(copyBtn);
    });

    expect(writeTextMock).toHaveBeenCalledWith(
      expect.stringContaining('David')
    );
    await waitFor(() => {
      expect(screen.getByText('Copied!')).toBeDefined();
    });
  });

  it('renders generic parameter fallback for unknown skill', () => {
    render(
      <SkillParameterDeck
        skillName="custom-unknown-skill"
        skillTitle="Custom Agent"
        onDispatch={vi.fn()}
      />
    );

    expect(screen.getByTestId('skill-param-input-instruction')).toBeDefined();
  });
});
