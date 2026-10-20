import type { GalaxyNode } from '../types/galaxy';

export type ArmsLayer = 'application' | 'routine' | 'skill' | 'system';

export interface SkillParameterDef {
  key: string;
  label: string;
  type: 'text' | 'date' | 'textarea' | 'select';
  placeholder?: string;
  defaultValue?: string;
  options?: string[];
  required?: boolean;
}

export interface ExecutableStationRoutine {
  routineName: string;
  title: string;
  category: 'deterministic' | 'ai_skill';
  armsLayer: ArmsLayer;
  isExecutable: boolean;
  description: string;
  schedule?: string;
  cliCommand?: string;
  platform?: string;
  skillParameters?: SkillParameterDef[];
}

/**
 * Returns dynamic parameter schema definitions tailored to specific AI skills.
 */
export function getSkillParameters(routineName: string): SkillParameterDef[] {
  const norm = routineName.toLowerCase();
  const todayStr = new Date().toISOString().split('T')[0];

  if (norm.includes('meeting') || norm.includes('preflight')) {
    return [
      {
        key: 'attendee',
        label: 'Meeting With / Attendees',
        type: 'text',
        placeholder: 'e.g. Alice, Bob, or Architecture Guild',
        required: true,
      },
      {
        key: 'topic',
        label: 'Key Topics & Agenda',
        type: 'text',
        placeholder: 'e.g. Q3 Architecture Review, API Gateway migration',
        required: true,
      },
      {
        key: 'date',
        label: 'Meeting Date',
        type: 'date',
        defaultValue: todayStr,
      },
      {
        key: 'context',
        label: 'Additional Context / Note Links',
        type: 'textarea',
        placeholder: 'Relevant notes, links, or context for the briefing...',
      },
    ];
  }

  if (norm.includes('dream')) {
    return [
      {
        key: 'date',
        label: 'Reference Date',
        type: 'date',
        defaultValue: todayStr,
      },
      {
        key: 'focus_domain',
        label: 'Focus Domain (Optional)',
        type: 'text',
        placeholder: 'e.g. notes/arquitetura, notes/pessoas',
      },
    ];
  }

  if (norm.includes('avd') || norm.includes('compiler') || norm.includes('dossier')) {
    return [
      {
        key: 'period',
        label: 'Review Period / Cycle',
        type: 'text',
        placeholder: 'e.g. 2026-H1, Last 90 Days',
        defaultValue: 'Last 90 Days',
        required: true,
      },
      {
        key: 'focus_areas',
        label: 'Strategic Focus Areas',
        type: 'textarea',
        placeholder: 'Key leadership initiatives, staff impact topics...',
      },
    ];
  }

  // Generic AI Skill fallback parameter
  return [
    {
      key: 'instruction',
      label: 'Context & Instructions',
      type: 'textarea',
      placeholder: 'Specific instructions, context, or parameters for this skill...',
    },
  ];
}

/**
 * Hydrates a complete structured prompt for an AI agent using skill parameters.
 */
export function generateSkillPrompt(
  skillName: string,
  params: Record<string, string>,
  title?: string
): string {
  const norm = skillName.toLowerCase();
  const dateStr = params.date || new Date().toISOString().split('T')[0];

  if (norm.includes('meeting') || norm.includes('preflight')) {
    const withWho = params.attendee || 'Partner / Team';
    const topic = params.topic || 'General Sync';
    const context = params.context ? `\n\nContext & References:\n${params.context}` : '';
    return `Execute meeting preflight briefing protocol for meeting with ${withWho} on "${topic}" (Date: ${dateStr}).${context}\n\nPlease analyze relevant vault notes, interaction histories in notes/people/, and prepare strategic discussion points, mutual goals, and open questions.`;
  }

  if (norm.includes('dream')) {
    const domain = params.focus_domain ? ` Focus on domain '${params.focus_domain}'.` : '';
    return `Execute cognitive REM dream cycle consolidation for date ${dateStr}.${domain}\n\nReview daily notes, triage inbox items, heal bi-directional links, and reconcile leadership time telemetry.`;
  }

  if (norm.includes('avd') || norm.includes('compiler')) {
    const period = params.period || 'Last 90 Days';
    const focus = params.focus_areas ? `\nFocus Areas:\n${params.focus_areas}` : '';
    return `Compile executive performance review and staff impact dossier for period ${period}.${focus}\n\nSynthesize accomplishments, architecture decisions, mentorship impact, and leadership archetypes.`;
  }

  const custom = params.instruction || params.prompt || '';
  const body = custom ? `\nInstructions:\n${custom}` : '';
  return `Execute AI skill '${title || skillName}' adhering to the protocol in .agents/skills/${skillName}/SKILL.md.${body}`.trim();
}

/**
 * Deterministically resolves a perimeter satellite node into an executable routine/skill descriptor.
 * Used by ArmsInspectorCard to provide in-drawer execution, specialized controls, and live streaming.
 */
export function resolveStationRoutine(station: Partial<Pick<GalaxyNode, 'id' | 'title' | 'description' | 'ring'>> | null | undefined): ExecutableStationRoutine | null {
  if (!station || !station.id) return null;

  const rawId = String(station.id).toLowerCase();
  const id = rawId.replace(/^orbit:/, '');

  // Known orbital satellites mapping
  const routineMap: Record<
    string,
    {
      routine: string;
      title: string;
      category: 'deterministic' | 'ai_skill';
      armsLayer: ArmsLayer;
      description: string;
      schedule?: string;
      cliCommand?: string;
      platform?: string;
    }
  > = {
    'routine:dream': {
      routine: 'dream',
      title: 'Dream Cycle (REM Consolidation)',
      category: 'deterministic',
      armsLayer: 'routine',
      schedule: 'Daily at 06:00 (On-Demand)',
      cliCommand: 'daneel dream',
      description: 'Hypnagogic Cognitive REM consolidation, Zettelkasten routing, and telemetry reconciliation',
    },
    'routine:tracking': {
      routine: 'log',
      title: 'Staff Time Tracking & Allocation',
      category: 'deterministic',
      armsLayer: 'routine',
      schedule: 'Continuous (7-Day Window)',
      cliCommand: 'daneel log --recent=7',
      description: 'Tracks time allocation across [A]-[E] leadership archetypes over the last 7 days',
    },
    'routine:radar': {
      routine: 'dossier',
      title: 'Strategic Radar & Executive Dossier',
      category: 'deterministic',
      armsLayer: 'routine',
      schedule: 'Bi-Weekly / Monthly',
      cliCommand: 'daneel dossier',
      description: 'Compiles executive performance review dossiers, active radar, and impact reports',
    },
    'harvester:jira': {
      routine: 'harvest:jira',
      title: 'Jira Harvester',
      category: 'deterministic',
      armsLayer: 'application',
      platform: 'jira',
      cliCommand: 'daneel harvest jira',
      description: 'Harvests active issues, tasks, and post-mortems from Jira Cloud',
    },
    'harvester:github': {
      routine: 'harvest:github',
      title: 'GitHub Harvester',
      category: 'deterministic',
      armsLayer: 'application',
      platform: 'github',
      cliCommand: 'daneel harvest github',
      description: 'Continuous ingestion of docs, ADRs, and RFCs from GitHub repositories',
    },
    'harvester:gitlab': {
      routine: 'harvest:gitlab',
      title: 'GitLab Harvester',
      category: 'deterministic',
      armsLayer: 'application',
      platform: 'gitlab',
      cliCommand: 'daneel harvest gitlab',
      description: 'Harvests personal user repositories from GitLab into vault source catalogs',
    },
    'harvester:gdrive': {
      routine: 'harvest:gdrive',
      title: 'Google Drive Harvester',
      category: 'deterministic',
      armsLayer: 'application',
      platform: 'gdrive',
      cliCommand: 'daneel harvest gdrive',
      description: 'Ingestion of strategic alignment docs, 1:1 notes, and meeting minutes',
    },
    'harvester:mcp': {
      routine: 'session_mining',
      title: 'Agent Session Miner & Evals',
      category: 'deterministic',
      armsLayer: 'application',
      platform: 'mcp',
      cliCommand: 'daneel audit sessions',
      description: 'Audits AI agent session transcripts for corrections, new skills, and evals',
    },
    'skill:vault-dream-cycle': {
      routine: 'vault-dream-cycle',
      title: 'AI Skill: vault-dream-cycle',
      category: 'ai_skill',
      armsLayer: 'skill',
      description: 'Autonomous cognitive consolidation and archetype protocol for AI agents',
    },
    'skill:vault-meetings': {
      routine: 'meeting-preflight',
      title: 'AI Skill: meeting-preflight',
      category: 'ai_skill',
      armsLayer: 'skill',
      description: 'Daily morning briefing, priorities alignment, and 1:1 prep generation',
    },
    'skill:vault-preflight': {
      routine: 'meeting-preflight',
      title: 'AI Skill: meeting-preflight',
      category: 'ai_skill',
      armsLayer: 'skill',
      description: 'Daily morning briefing, priorities alignment, and 1:1 prep generation',
    },
    'skill:vault-tracking': {
      routine: 'log',
      title: 'AI Skill: vault-tracking',
      category: 'ai_skill',
      armsLayer: 'skill',
      description: 'Reconciliation and calculation of leadership and execution hours',
    },
  };

  if (routineMap[id]) {
    const item = routineMap[id];
    return {
      routineName: item.routine,
      title: station.title || item.title,
      category: item.category,
      armsLayer: item.armsLayer,
      isExecutable: true,
      description: station.description || item.description,
      schedule: item.schedule,
      cliCommand: item.cliCommand,
      platform: item.platform,
      skillParameters: item.armsLayer === 'skill' ? getSkillParameters(item.routine) : undefined,
    };
  }

  // Explicit title match for Jira Harvester
  if (
    id === 'jira' ||
    id === 'jira-harvester' ||
    (station.title && station.title.toLowerCase().includes('jira harvester'))
  ) {
    return {
      routineName: 'harvest:jira',
      title: station.title || 'Jira Harvester',
      category: 'deterministic',
      armsLayer: 'application',
      platform: 'jira',
      cliCommand: 'daneel harvest jira',
      isExecutable: true,
      description: station.description || 'Harvests active issues, tasks, and post-mortems from Jira Cloud',
    };
  }

  // Dynamic prefix matching
  if (id.startsWith('skill:')) {
    const rawName = id.replace('skill:', '');
    return {
      routineName: rawName,
      title: station.title || `AI Skill: ${rawName}`,
      category: 'ai_skill',
      armsLayer: 'skill',
      isExecutable: true,
      description: station.description || `Autonomous agent skill in .agents/skills/${rawName}`,
      skillParameters: getSkillParameters(rawName),
    };
  }

  if (id.startsWith('routine:')) {
    const rawName = id.replace('routine:', '');
    return {
      routineName: rawName,
      title: station.title || `Routine: ${rawName}`,
      category: 'deterministic',
      armsLayer: 'routine',
      isExecutable: true,
      schedule: 'Scheduled / On-Demand',
      cliCommand: `daneel ${rawName}`,
      description: station.description || `Deterministic routine '${rawName}'`,
    };
  }

  if (id.startsWith('harvester:')) {
    const rawPlatform = id.replace('harvester:', '');
    if (rawPlatform === 'mcp') {
      return {
        routineName: 'session_mining',
        title: station.title || 'Agent Session Miner & Evals',
        category: 'deterministic',
        armsLayer: 'application',
        platform: 'mcp',
        cliCommand: 'daneel audit sessions',
        isExecutable: true,
        description: station.description || 'Audits AI agent session transcripts for corrections, new skills, and evals',
      };
    }
    return {
      routineName: `harvest:${rawPlatform}`,
      title: station.title || `${rawPlatform.toUpperCase()} Harvester`,
      category: 'deterministic',
      armsLayer: 'application',
      platform: rawPlatform,
      cliCommand: `daneel harvest ${rawPlatform}`,
      isExecutable: true,
      description: station.description || `Documentation harvester satellite pipeline for ${rawPlatform}`,
    };
  }

  // Fallback by orbital ring
  if (station.ring === 'skills' || station.ring === '1') {
    return {
      routineName: id,
      title: station.title || `AI Skill: ${id}`,
      category: 'ai_skill',
      armsLayer: 'skill',
      isExecutable: true,
      description: station.description || 'Autonomous agent capability',
      skillParameters: getSkillParameters(id),
    };
  }

  if (station.ring === 'routines' || station.ring === '2') {
    return {
      routineName: id,
      title: station.title || `Routine: ${id}`,
      category: 'deterministic',
      armsLayer: 'routine',
      isExecutable: true,
      schedule: 'Scheduled / On-Demand',
      cliCommand: `daneel ${id}`,
      description: station.description || 'Deterministic native maintenance routine',
    };
  }

  if (station.ring === 'harvesters' || station.ring === '3') {
    return {
      routineName: 'harvest',
      title: station.title || 'Documentation Harvester',
      category: 'deterministic',
      armsLayer: 'application',
      cliCommand: 'daneel harvest',
      isExecutable: true,
      description: station.description || 'Documentation harvester pipeline',
    };
  }

  return null;
}
