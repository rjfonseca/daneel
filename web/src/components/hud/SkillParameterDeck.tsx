import React, { useState, useEffect, useMemo } from 'react';
import { 
  Sparkles, 
  Copy, 
  Check, 
  ChevronDown, 
  ChevronUp, 
  FileCode, 
  Loader2 
} from 'lucide-react';
import { CyberButton } from './CyberButton';
import { NeonBadge } from './NeonBadge';
import { 
  getSkillParameters, 
  generateSkillPrompt 
} from '@/lib/stationResolver';
import { cn } from '@/lib/utils';

export interface SkillParameterDeckProps {
  skillName: string;
  skillTitle?: string;
  isRunning?: boolean;
  onDispatch: (params: Record<string, string>) => void;
  compact?: boolean;
  className?: string;
  testIdPrefix?: string;
}

export const SkillParameterDeck: React.FC<SkillParameterDeckProps> = ({
  skillName,
  skillTitle,
  isRunning = false,
  onDispatch,
  compact = false,
  className,
  testIdPrefix = 'skill-param',
}) => {
  const [params, setParams] = useState<Record<string, string>>({});
  const [copied, setCopied] = useState(false);
  const [showPreview, setShowPreview] = useState(false);

  const parameterDefs = useMemo(() => getSkillParameters(skillName), [skillName]);

  // Re-initialize parameters when skill changes
  useEffect(() => {
    const initial: Record<string, string> = {};
    for (const def of parameterDefs) {
      if (def.defaultValue) {
        initial[def.key] = def.defaultValue;
      } else {
        initial[def.key] = '';
      }
    }
    setParams(initial);
    setCopied(false);
  }, [skillName, parameterDefs]);

  const promptText = useMemo(
    () => generateSkillPrompt(skillName, params, skillTitle),
    [skillName, params, skillTitle]
  );

  const handleInputChange = (key: string, value: string) => {
    setParams((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const handleCopyPrompt = async () => {
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(promptText);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.warn('Failed to copy prompt to clipboard:', err);
    }
  };

  return (
    <div 
      data-testid={`${testIdPrefix}-deck`}
      className={cn(
        "rounded-xl border border-purple-500/30 bg-[#070a14]/80 backdrop-blur-md p-3 sm:p-4 shadow-[0_0_20px_rgba(168,85,247,0.12)] space-y-3",
        className
      )}
    >
      {/* Header Deck Info */}
      <div className="flex items-center justify-between gap-2 border-b border-purple-500/20 pb-2.5">
        <div className="flex items-center gap-2 min-w-0">
          <div className="p-1.5 rounded-lg bg-purple-500/20 text-purple-300 border border-purple-500/40">
            <Sparkles className="w-3.5 h-3.5" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono font-bold text-white uppercase truncate">
                Agent Parameter Protocol
              </span>
              <NeonBadge accent="purple" size="sm">
                AI SKILL
              </NeonBadge>
            </div>
            <p className="text-[11px] text-slate-400 font-mono truncate">
              {skillTitle || skillName}
            </p>
          </div>
        </div>

        {/* Copy Prompt & Preview Actions */}
        <div className="flex items-center gap-1.5 shrink-0">
          <CyberButton
            onClick={handleCopyPrompt}
            variant="ghost"
            size="sm"
            data-testid={`${testIdPrefix}-copy-prompt-btn`}
            className="text-purple-300 hover:text-white hover:bg-purple-950/40 border border-purple-500/20 px-2 py-1 text-[11px] gap-1"
            title="Copy hydrated prompt to clipboard"
          >
            {copied ? (
              <>
                <Check className="w-3 h-3 text-emerald-400" />
                <span>Copied!</span>
              </>
            ) : (
              <>
                <Copy className="w-3 h-3" />
                <span>Copy Prompt</span>
              </>
            )}
          </CyberButton>

          <button
            onClick={() => setShowPreview(!showPreview)}
            data-testid={`${testIdPrefix}-toggle-preview-btn`}
            aria-expanded={showPreview}
            className="p-1 rounded-sm bg-surface text-slate-400 hover:text-white border border-white/10 transition-colors cursor-pointer cosmic-focus-visible"
            title="Toggle prompt preview"
          >
            {showPreview ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
          </button>
        </div>
      </div>

      {/* Dynamic Form Parameter Fields */}
      <div className={cn("grid gap-2.5", compact ? "grid-cols-1" : "grid-cols-1 sm:grid-cols-2")}>
        {parameterDefs.map((param) => {
          const isFullWidth = param.type === 'textarea' || !compact && parameterDefs.length % 2 !== 0 && param === parameterDefs[parameterDefs.length - 1];

          return (
            <div 
              key={param.key} 
              className={cn("space-y-1", isFullWidth ? "sm:col-span-2" : "")}
            >
              <label 
                htmlFor={`skill-input-${param.key}`}
                className="block text-[11px] font-mono text-purple-300/90 font-medium"
              >
                {param.label}
                {param.required && <span className="text-rose-400 ml-1">*</span>}
              </label>

              {param.type === 'textarea' ? (
                <textarea
                  id={`skill-input-${param.key}`}
                  rows={compact ? 2 : 3}
                  value={params[param.key] || ''}
                  onChange={(e) => handleInputChange(param.key, e.target.value)}
                  placeholder={param.placeholder}
                  data-testid={`${testIdPrefix}-input-${param.key}`}
                  className="w-full text-xs font-mono bg-[#04060d] border border-purple-500/30 rounded-lg p-2 text-slate-200 placeholder-slate-600 focus:outline-hidden focus:border-purple-400 transition-colors resize-none selection:bg-purple-500/40 selection:text-white"
                />
              ) : param.type === 'select' && param.options ? (
                <select
                  id={`skill-input-${param.key}`}
                  value={params[param.key] || ''}
                  onChange={(e) => handleInputChange(param.key, e.target.value)}
                  data-testid={`${testIdPrefix}-input-${param.key}`}
                  className="w-full text-xs font-mono bg-[#04060d] border border-purple-500/30 rounded-lg p-2 text-slate-200 focus:outline-hidden focus:border-purple-400 transition-colors"
                >
                  {param.options.map((opt) => (
                    <option key={opt} value={opt}>
                      {opt}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  id={`skill-input-${param.key}`}
                  type={param.type === 'date' ? 'date' : 'text'}
                  value={params[param.key] || ''}
                  onChange={(e) => handleInputChange(param.key, e.target.value)}
                  placeholder={param.placeholder}
                  data-testid={`${testIdPrefix}-input-${param.key}`}
                  className="w-full text-xs font-mono bg-[#04060d] border border-purple-500/30 rounded-lg p-2 text-slate-200 placeholder-slate-600 focus:outline-hidden focus:border-purple-400 transition-colors selection:bg-purple-500/40 selection:text-white"
                />
              )}
            </div>
          );
        })}
      </div>

      {/* Expandable Prompt Preview */}
      {showPreview && (
        <div 
          data-testid={`${testIdPrefix}-prompt-preview`}
          className="rounded-lg border border-purple-500/20 bg-[#03050a] p-3 text-[11px] font-mono text-slate-300 space-y-1.5 animate-in fade-in duration-150"
        >
          <div className="flex items-center gap-1.5 text-purple-400 text-[10px] uppercase font-bold tracking-wider">
            <FileCode className="w-3 h-3" />
            <span>Structured Prompt Payload Preview</span>
          </div>
          <pre className="whitespace-pre-wrap select-text leading-relaxed text-slate-300">
            {promptText}
          </pre>
        </div>
      )}

      {/* Dispatch Action Button Row */}
      <div className="pt-1 flex items-center justify-between gap-2">
        <span className="text-[10px] font-mono text-slate-500">
          Dispatches autonomous execution in headless runner
        </span>

        <CyberButton
          onClick={() => onDispatch(params)}
          disabled={isRunning}
          variant="hud"
          accent="purple"
          size="sm"
          data-testid={`${testIdPrefix}-dispatch-btn`}
          className="gap-1.5 px-3 py-1.5 text-xs shrink-0 font-bold"
        >
          {isRunning ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin text-purple-300" />
          ) : (
            <Sparkles className="w-3.5 h-3.5 text-purple-300" />
          )}
          <span>{isRunning ? 'Executing Agent...' : 'Dispatch Agent'}</span>
        </CyberButton>
      </div>
    </div>
  );
};
