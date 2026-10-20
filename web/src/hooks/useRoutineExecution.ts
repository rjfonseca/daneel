import { useState, useRef, useCallback } from 'react';
import { client } from '@/lib/apiClient';

export interface UseRoutineExecutionResult {
  isRunning: boolean;
  runningRoutineName: string | null;
  logs: string[];
  verbose: boolean;
  setVerbose: (v: boolean | ((prev: boolean) => boolean)) => void;
  activeExitCode: number | null;
  executeRoutine: (
    routineName: string,
    customVerbose?: boolean,
    params?: Record<string, string>
  ) => Promise<void>;
  abortRoutine: () => void;
  clearLogs: () => void;
}

/**
 * Shared hook managing headless ConnectRPC routine execution and live streaming output.
 * Shared between CockpitModal and ArmsInspectorCard.
 */
export function useRoutineExecution(): UseRoutineExecutionResult {
  const [isRunning, setIsRunning] = useState<boolean>(false);
  const [runningRoutineName, setRunningRoutineName] = useState<string | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [verbose, setVerbose] = useState<boolean>(false);
  const [activeExitCode, setActiveExitCode] = useState<number | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);

  const executeRoutine = useCallback(
    async (
      routineName: string,
      customVerbose?: boolean,
      params?: Record<string, string>
    ) => {
      const trimmed = routineName.trim();
      if (!trimmed || isRunning) return;

      const isVerb = customVerbose !== undefined ? customVerbose : verbose;
      const controller = new AbortController();
      abortControllerRef.current = controller;

      setIsRunning(true);
      setRunningRoutineName(trimmed);
      setActiveExitCode(null);

      const initialTimestamp = new Date().toLocaleTimeString();
      setLogs((prev) => [
        ...prev,
        `────────────────────────────────────────────────────────────────────────────────`,
        `🚀 [${initialTimestamp}] Initializing routine '${trimmed}'...`,
        `⚙️ Mode: ${isVerb ? 'Verbose (--verbose)' : 'Deterministic Standard'}`,
      ]);

      try {
        const stream = client.executeRoutine(
          {
            routineName: trimmed,
            verbose: isVerb,
            params: params || {},
          },
          { signal: controller.signal }
        );

        for await (const event of stream) {
          if (event.line) {
            setLogs((prev) => [...prev, event.line]);
          }
          if (event.isCompleted) {
            setActiveExitCode(event.exitCode);
            const finishTimestamp = new Date().toLocaleTimeString();
            if (event.exitCode === 0) {
              setLogs((prev) => [
                ...prev,
                `✨ [${finishTimestamp}] Routine '${trimmed}' completed successfully (Exit Code: 0).`,
              ]);
            } else {
              setLogs((prev) => [
                ...prev,
                `❌ [${finishTimestamp}] Routine '${trimmed}' exited with status code ${event.exitCode}.`,
              ]);
            }
          }
        }
      } catch (err: unknown) {
        const errorMsg = err instanceof Error ? err.message : String(err);
        const isAbort = (err instanceof Error && err.name === 'AbortError') || controller.signal.aborted;
        if (isAbort) {
          setLogs((prev) => [...prev, `⚠️ Routine '${trimmed}' was manually cancelled by operator.`]);
        } else {
          setLogs((prev) => [...prev, `❌ [Streaming Error]: ${errorMsg}`]);
        }
      } finally {
        setIsRunning(false);
        setRunningRoutineName(null);
        abortControllerRef.current = null;
      }
    },
    [isRunning, verbose]
  );

  const abortRoutine = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
  }, []);

  const clearLogs = useCallback(() => {
    setLogs([]);
    setActiveExitCode(null);
  }, []);

  return {
    isRunning,
    runningRoutineName,
    logs,
    verbose,
    setVerbose,
    activeExitCode,
    executeRoutine,
    abortRoutine,
    clearLogs,
  };
}
