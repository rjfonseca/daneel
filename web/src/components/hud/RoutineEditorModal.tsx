import React, { useState, useEffect } from 'react';
import { X, Save, Clock, Terminal, AlertCircle, Loader2 } from 'lucide-react';
import { client } from '@/lib/apiClient';
import { HudCard } from './HudCard';
import { CyberButton } from './CyberButton';
import type { RoutineStatusItem } from '@/gen/api/v1/graph_pb';

export interface RoutineEditorModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (routineName: string) => void;
  initialRoutine?: RoutineStatusItem | null;
}

const SCHEDULE_PRESETS = [
  { label: 'Hourly', value: 'hourly' },
  { label: 'Daily (06:00)', value: 'daily' },
  { label: 'Weekly (Mon 07:00)', value: 'weekly' },
  { label: 'Every 6 Hours', value: '0 */6 * * *' },
  { label: 'Weekdays 09:00', value: '0 9 * * 1-5' },
];

export const RoutineEditorModal: React.FC<RoutineEditorModalProps> = ({
  isOpen,
  onClose,
  onSaved,
  initialRoutine,
}) => {
  const isEditing = !!initialRoutine && !!initialRoutine.name;

  const [name, setName] = useState('');
  const [command, setCommand] = useState('');
  const [schedule, setSchedule] = useState('hourly');
  const [description, setDescription] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    if (initialRoutine) {
      setName(initialRoutine.name);
      setCommand(initialRoutine.command || initialRoutine.name);
      setSchedule(initialRoutine.schedule || 'hourly');
      setDescription(initialRoutine.description || '');
      setEnabled(initialRoutine.enabled ?? true);
    } else {
      setName('');
      setCommand('');
      setSchedule('hourly');
      setDescription('');
      setEnabled(true);
    }
    setErrorMessage(null);
  }, [initialRoutine, isOpen]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim().toLowerCase();
    if (!trimmedName) {
      setErrorMessage('Routine name is required');
      return;
    }
    const trimmedCommand = command.trim() || trimmedName;
    const trimmedSchedule = schedule.trim() || 'daily';

    setIsSaving(true);
    setErrorMessage(null);

    try {
      const res = await client.upsertRoutine({
        name: trimmedName,
        command: trimmedCommand,
        schedule: trimmedSchedule,
        description: description.trim(),
        enabled,
      });

      if (!res.success) {
        setErrorMessage(res.error || 'Failed to save routine');
        setIsSaving(false);
        return;
      }

      onSaved(trimmedName);
      onClose();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : 'An error occurred while saving');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-60 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-in fade-in duration-150"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="routine-editor-title"
      data-testid="routine-editor-modal"
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg"
      >
        <HudCard
          variant="cyan"
          elevation="elevated"
          techCorners
          className="w-full flex flex-col overflow-hidden shadow-hud-deep border-cyan-500/40"
        >
          {/* Header */}
          <div className="flex items-center justify-between p-4 border-b border-white/10 bg-[#070a14]/80">
            <div className="flex items-center gap-2.5">
              <div className="p-1.5 rounded-sm bg-cyan-500/20 text-cyan-300 border border-cyan-500/40">
                <Clock className="w-4 h-4" />
              </div>
              <div>
                <h3 id="routine-editor-title" className="text-sm font-bold text-white font-mono tracking-wide">
                  {isEditing ? `EDIT ROUTINE: ${initialRoutine?.name}` : 'CREATE NEW SCHEDULED ROUTINE'}
                </h3>
                <p className="text-[11px] text-slate-400">
                  Systemd User Timer & CLI Execution Management
                </p>
              </div>
            </div>

            <CyberButton
              onClick={onClose}
              variant="ghost"
              size="icon"
              aria-label="Close Editor"
              className="text-slate-400 hover:text-white"
            >
              <X className="w-4 h-4" />
            </CyberButton>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="p-4 sm:p-5 space-y-4 bg-void/90 font-mono text-xs">
            {errorMessage && (
              <div className="flex items-start gap-2 p-2.5 rounded-sm bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
                <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
                <span>{errorMessage}</span>
              </div>
            )}

            {/* Name */}
            <div>
              <label className="block text-slate-300 mb-1 font-semibold">
                Routine Identifier <span className="text-cyan-400">*</span>
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={isEditing}
                placeholder="e.g. tidy, backup, sync-issues"
                data-testid="routine-input-name"
                className="w-full px-3 py-2 bg-surface border border-white/10 rounded-sm text-white placeholder-slate-500 focus:outline-hidden focus:border-cyan-500/60 disabled:opacity-50 disabled:cursor-not-allowed"
                required
              />
              <span className="text-[10px] text-slate-500 mt-1 block">
                Lowercase alphanumeric identifier used for systemd service and timer naming.
              </span>
            </div>

            {/* Command */}
            <div>
              <label className="block text-slate-300 mb-1 font-semibold">
                CLI Command <span className="text-cyan-400">*</span>
              </label>
              <div className="relative">
                <Terminal className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
                <input
                  type="text"
                  value={command}
                  onChange={(e) => setCommand(e.target.value)}
                  placeholder="e.g. tidy, harvest, doctor, index (defaults to name)"
                  data-testid="routine-input-command"
                  className="w-full pl-9 pr-3 py-2 bg-surface border border-white/10 rounded-sm text-white placeholder-slate-500 focus:outline-hidden focus:border-cyan-500/60"
                />
              </div>
            </div>

            {/* Schedule */}
            <div>
              <label className="block text-slate-300 mb-1 font-semibold">
                Schedule Expression <span className="text-cyan-400">*</span>
              </label>
              <input
                type="text"
                value={schedule}
                onChange={(e) => setSchedule(e.target.value)}
                placeholder="hourly, daily, or cron format (0 */6 * * *)"
                data-testid="routine-input-schedule"
                className="w-full px-3 py-2 bg-surface border border-white/10 rounded-sm text-white placeholder-slate-500 focus:outline-hidden focus:border-cyan-500/60"
                required
              />
              {/* Presets */}
              <div className="flex flex-wrap gap-1.5 mt-2">
                {SCHEDULE_PRESETS.map((p) => (
                  <button
                    key={p.value}
                    type="button"
                    onClick={() => setSchedule(p.value)}
                    className={`px-2 py-0.5 rounded-sm text-[10px] border transition-colors cursor-pointer ${
                      schedule === p.value
                        ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40'
                        : 'bg-surface text-slate-400 border-white/5 hover:border-white/20'
                    }`}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Description */}
            <div>
              <label className="block text-slate-300 mb-1 font-semibold">
                Description
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe what this scheduled routine performs..."
                rows={2}
                data-testid="routine-input-description"
                className="w-full px-3 py-2 bg-surface border border-white/10 rounded-sm text-white placeholder-slate-500 focus:outline-hidden focus:border-cyan-500/60 resize-none"
              />
            </div>

            {/* Enabled Toggle */}
            <div className="flex items-center justify-between p-3 rounded-sm bg-surface border border-white/5">
              <div>
                <span className="text-xs font-semibold text-white block">Enable Systemd Timer</span>
                <span className="text-[10px] text-slate-400 block">
                  Activate automatic background execution on schedule
                </span>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={enabled}
                  onChange={(e) => setEnabled(e.target.checked)}
                  data-testid="routine-input-enabled"
                  className="sr-only peer"
                />
                <div className="w-9 h-5 bg-slate-800 peer-focus:outline-hidden rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-cyan-600"></div>
              </label>
            </div>

            {/* Footer Buttons */}
            <div className="flex items-center justify-end gap-2 pt-3 border-t border-white/10">
              <CyberButton
                type="button"
                onClick={onClose}
                variant="ghost"
                size="sm"
                disabled={isSaving}
              >
                Cancel
              </CyberButton>
              <CyberButton
                type="submit"
                variant="hud"
                accent="cyan"
                size="sm"
                disabled={isSaving}
                data-testid="routine-save-btn"
                className="flex items-center gap-1.5"
              >
                {isSaving ? (
                  <>
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    <span>Saving...</span>
                  </>
                ) : (
                  <>
                    <Save className="w-3.5 h-3.5" />
                    <span>{isEditing ? 'Save Changes' : 'Create Routine'}</span>
                  </>
                )}
              </CyberButton>
            </div>
          </form>
        </HudCard>
      </div>
    </div>
  );
};
