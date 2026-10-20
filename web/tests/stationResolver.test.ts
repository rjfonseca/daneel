import { describe, it, expect } from 'vitest';
import { 
  resolveStationRoutine, 
  getSkillParameters, 
  generateSkillPrompt 
} from '../src/lib/stationResolver';

describe('stationResolver', () => {
  it('returns null for empty or invalid station', () => {
    expect(resolveStationRoutine(null)).toBeNull();
    expect(resolveStationRoutine({})).toBeNull();
    expect(resolveStationRoutine({ id: '' })).toBeNull();
  });

  it('maps known routine satellites to deterministic routines with routine ARMS layer', () => {
    const dreamSat = resolveStationRoutine({ id: 'routine:dream' });
    expect(dreamSat).not.toBeNull();
    expect(dreamSat?.routineName).toBe('dream');
    expect(dreamSat?.category).toBe('deterministic');
    expect(dreamSat?.armsLayer).toBe('routine');
    expect(dreamSat?.isExecutable).toBe(true);
    expect(dreamSat?.schedule).toBeDefined();

    const trackingSat = resolveStationRoutine({ id: 'routine:tracking' });
    expect(trackingSat?.routineName).toBe('log');
    expect(trackingSat?.armsLayer).toBe('routine');

    const radarSat = resolveStationRoutine({ id: 'routine:radar' });
    expect(radarSat?.routineName).toBe('dossier');
    expect(radarSat?.armsLayer).toBe('routine');
  });

  it('maps known harvesters to platform-specific routines with application ARMS layer', () => {
    const jira = resolveStationRoutine({ id: 'harvester:jira', title: 'Jira Harvester' });
    expect(jira?.routineName).toBe('harvest:jira');
    expect(jira?.category).toBe('deterministic');
    expect(jira?.armsLayer).toBe('application');
    expect(jira?.platform).toBe('jira');

    const jiraOrbit = resolveStationRoutine({ id: 'orbit:harvester:jira' });
    expect(jiraOrbit?.routineName).toBe('harvest:jira');

    const jiraTitle = resolveStationRoutine({ id: 'custom-node', title: '🔷 Jira Harvester' });
    expect(jiraTitle?.routineName).toBe('harvest:jira');

    const gh = resolveStationRoutine({ id: 'harvester:github' });
    expect(gh?.routineName).toBe('harvest:github');
    expect(gh?.armsLayer).toBe('application');

    const gl = resolveStationRoutine({ id: 'harvester:gitlab' });
    expect(gl?.routineName).toBe('harvest:gitlab');

    const gd = resolveStationRoutine({ id: 'harvester:gdrive' });
    expect(gd?.routineName).toBe('harvest:gdrive');

    const mcp = resolveStationRoutine({ id: 'harvester:mcp' });
    expect(mcp?.routineName).toBe('session_mining');
    expect(mcp?.armsLayer).toBe('application');
  });

  it('maps known skill satellites to AI skills with skill ARMS layer and parameter schema', () => {
    const dreamSkill = resolveStationRoutine({ id: 'skill:vault-dream-cycle' });
    expect(dreamSkill?.routineName).toBe('vault-dream-cycle');
    expect(dreamSkill?.category).toBe('ai_skill');
    expect(dreamSkill?.armsLayer).toBe('skill');

    const meetingsSkill = resolveStationRoutine({ id: 'skill:vault-meetings' });
    expect(meetingsSkill?.routineName).toBe('meeting-preflight');
    expect(meetingsSkill?.category).toBe('ai_skill');
    expect(meetingsSkill?.armsLayer).toBe('skill');
    expect(meetingsSkill?.skillParameters).toBeDefined();
    expect(meetingsSkill?.skillParameters?.length).toBeGreaterThan(0);
  });

  it('dynamically handles custom skill: or routine: prefixes', () => {
    const customSkill = resolveStationRoutine({ id: 'skill:custom-agent' });
    expect(customSkill?.routineName).toBe('custom-agent');
    expect(customSkill?.category).toBe('ai_skill');
    expect(customSkill?.armsLayer).toBe('skill');

    const customRoutine = resolveStationRoutine({ id: 'routine:doctor' });
    expect(customRoutine?.routineName).toBe('doctor');
    expect(customRoutine?.category).toBe('deterministic');
    expect(customRoutine?.armsLayer).toBe('routine');
  });

  it('handles fallback by ring', () => {
    const ringSkill = resolveStationRoutine({ id: 'my-custom-skill', ring: 'skills' });
    expect(ringSkill?.routineName).toBe('my-custom-skill');
    expect(ringSkill?.category).toBe('ai_skill');
    expect(ringSkill?.armsLayer).toBe('skill');

    const ringRoutine = resolveStationRoutine({ id: 'my-custom-routine', ring: 'routines' });
    expect(ringRoutine?.routineName).toBe('my-custom-routine');
    expect(ringRoutine?.category).toBe('deterministic');
    expect(ringRoutine?.armsLayer).toBe('routine');
  });

  it('getSkillParameters returns context-specific parameters for meeting preflight', () => {
    const params = getSkillParameters('meeting-preflight');
    expect(params.some(p => p.key === 'attendee')).toBe(true);
    expect(params.some(p => p.key === 'topic')).toBe(true);
    expect(params.some(p => p.key === 'date')).toBe(true);
  });

  it('generateSkillPrompt hydrates prompt with given parameters', () => {
    const prompt = generateSkillPrompt('meeting-preflight', {
      attendee: 'Carlos',
      topic: 'Q3 Architecture',
      date: '2026-09-20',
      context: 'See notes/arquitetura/gateway.md',
    });

    expect(prompt).toContain('Carlos');
    expect(prompt).toContain('Q3 Architecture');
    expect(prompt).toContain('2026-09-20');
    expect(prompt).toContain('See notes/arquitetura/gateway.md');
  });
});
