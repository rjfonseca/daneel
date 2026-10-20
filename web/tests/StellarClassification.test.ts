import { describe, it, expect } from 'vitest';
import {
  classifyStar,
  STELLAR_SPECTRUM_LEGEND
} from '../src/lib/stellarClassification';

describe('StellarClassification', () => {
  const NOW = 1773000000; // Fixed timestamp in seconds

  describe('classifyStar', () => {
    it('classifies Quasars for super-hubs (connections >= 15) modified recently (<= 7 days)', () => {
      const quasarNode = {
        id: 'notes/arquitetura/hexagonal.md',
        category: 'arquitetura',
        modTime: NOW - 2 * 86400, // 2 days ago
        connections: 28,
      };

      const result = classifyStar(quasarNode, NOW);
      expect(result.classification).toBe('QUASAR');
      expect(result.badge).toBe('⚡ QUASAR');
      expect(result.color).toBe('#38bdf8');
      expect(result.size).toBe(2.5);
    });

    it('classifies Pulsars for recent daily notes (<= 3 days)', () => {
      const pulsarNode1 = {
        id: 'daily/2026-09-09.md',
        category: 'daily',
        modTime: NOW - 0.5 * 86400, // 12 hours ago
        connections: 5,
      };
      const pulsarNode2 = {
        id: 'daily/2026-09-08.md',
        category: 'root', // even if category is root, id starts with daily/
        modTime: NOW - 1.5 * 86400,
        connections: 2,
      };

      expect(classifyStar(pulsarNode1, NOW).classification).toBe('PULSAR');
      expect(classifyStar(pulsarNode2, NOW).classification).toBe('PULSAR');
      expect(classifyStar(pulsarNode1, NOW).badge).toBe('📡 PULSAR');
    });

    it('does not classify old daily notes as Pulsars', () => {
      const oldDaily = {
        id: 'daily/2026-01-01.md',
        category: 'daily',
        modTime: NOW - 60 * 86400, // 60 days ago
        connections: 1,
      };

      const result = classifyStar(oldDaily, NOW);
      expect(result.classification).toBe('BROWN_DWARF');
    });

    it('classifies Blue Giants (O/B) for hot young notes modified within 3 days', () => {
      const freshNote = {
        id: 'notes/ia/gemini-flash.md',
        category: 'ia',
        modTime: NOW - 1 * 86400, // 1 day ago
        connections: 4,
      };

      const result = classifyStar(freshNote, NOW);
      expect(result.classification).toBe('BLUE_GIANT');
      expect(result.badge).toBe('💎 BLUE GIANT');
      expect(result.color).toBe('#60a5fa');
      expect(result.size).toBe(1.5);
    });

    it('classifies White/Cyan Stars (A/F) for notes modified between 3 and 7 days', () => {
      const weekNote = {
        id: 'notes/go/concurrency.md',
        category: 'go',
        modTime: NOW - 5 * 86400, // 5 days ago
        connections: 6,
      };

      const result = classifyStar(weekNote, NOW);
      expect(result.classification).toBe('WHITE_CYAN');
      expect(result.badge).toBe('✨ WHITE STAR');
      expect(result.color).toBe('#a5f3fc');
      expect(result.size).toBe(1.1);
    });

    it('classifies Yellow Suns (G) for notes modified between 7 and 20 days', () => {
      const matureNote = {
        id: 'notes/carreira/staff-engine.md',
        category: 'carreira',
        modTime: NOW - 14 * 86400, // 14 days ago
        connections: 8,
      };

      const result = classifyStar(matureNote, NOW);
      expect(result.classification).toBe('YELLOW_SUN');
      expect(result.badge).toBe('☀️ YELLOW DWARF');
      expect(result.color).toBe('#facc15');
      expect(result.size).toBe(0.85);
    });

    it('classifies Orange Stars (K) for mature notes (> 20 days) with at least 3 connections', () => {
      const connectedOldNote = {
        id: 'notes/sistemas/kafka.md',
        category: 'sistemas',
        modTime: NOW - 45 * 86400, // 45 days ago
        connections: 5,
      };

      const result = classifyStar(connectedOldNote, NOW);
      expect(result.classification).toBe('ORANGE_STAR');
      expect(result.badge).toBe('🟠 ORANGE STAR');
      expect(result.color).toBe('#fb923c');
      expect(result.size).toBe(0.75);
    });

    it('classifies Red Dwarfs (M) for mature notes (> 20 days) with exactly 2 connections', () => {
      const stableReference = {
        id: 'notes/padroes/solid.md',
        category: 'padroes',
        modTime: NOW - 90 * 86400,
        connections: 2,
      };

      const result = classifyStar(stableReference, NOW);
      expect(result.classification).toBe('RED_DWARF');
      expect(result.badge).toBe('🔴 RED DWARF');
      expect(result.color).toBe('#fb7185');
      expect(result.size).toBe(0.60);
    });

    it('classifies Brown Dwarfs (L/T/Y) for mature notes (> 20 days) with <= 1 connection', () => {
      const leafNode = {
        id: 'notes/snippets/curl.md',
        category: 'snippets',
        modTime: NOW - 120 * 86400,
        connections: 1,
      };
      const isolatedNode = {
        id: 'notes/rascunhos/ideia.md',
        category: 'rascunhos',
        modTime: NOW - 30 * 86400,
        connections: 0,
      };

      expect(classifyStar(leafNode, NOW).classification).toBe('BROWN_DWARF');
      expect(classifyStar(isolatedNode, NOW).classification).toBe('BROWN_DWARF');
      expect(classifyStar(leafNode, NOW).badge).toBe('🤎 BROWN DWARF');
      expect(classifyStar(leafNode, NOW).color).toBe('#d97706');
      expect(classifyStar(leafNode, NOW).size).toBe(0.40);
      expect(classifyStar(leafNode, NOW).alpha).toBe(0.80);
    });

    it('handles millisecond timestamps gracefully (> 1e11)', () => {
      const msTimestamp = (NOW - 2 * 86400) * 1000;
      const node = {
        id: 'notes/test.md',
        modTime: msTimestamp,
        connections: 1,
      };

      const result = classifyStar(node, NOW);
      expect(result.classification).toBe('BLUE_GIANT');
    });

    it('defaults undefined or missing modTime to Brown Dwarf if connections <= 1', () => {
      const noTimeNode = {
        id: 'notes/legacy.md',
        connections: 1,
      };

      const result = classifyStar(noTimeNode, NOW);
      expect(result.classification).toBe('BROWN_DWARF');
    });

    it('classifies ARMS orbital stations with specialized badges and colors', () => {
      const harvester = {
        id: 'harvester:github',
        ring: 'harvesters',
      };
      const routine = {
        id: 'routine:staff-tracking',
        ring: 'routines',
      };
      const skill = {
        id: 'skill:dream-cycle',
        ring: 'skills',
      };

      const hRes = classifyStar(harvester, NOW);
      expect(hRes.badge).toBe('🛸 HARVESTER');
      expect(hRes.color).toBe('#10b981');
      expect(hRes.spectralClass).toBe('ARMS-III');

      const rRes = classifyStar(routine, NOW);
      expect(rRes.badge).toBe('⏱️ ROUTINE');
      expect(rRes.color).toBe('#f59e0b');
      expect(rRes.spectralClass).toBe('ARMS-II');

      const sRes = classifyStar(skill, NOW);
      expect(sRes.badge).toBe('🧠 SKILL');
      expect(sRes.color).toBe('#a855f7');
      expect(sRes.spectralClass).toBe('ARMS-I');
    });

    it('classifies Cluster Core (Alpha Star) for repository index nodes', () => {
      const repoMocNode = {
        id: 'sources/gitlab/acme/saas/pocs/my-repo/index.md',
        category: 'sources/gitlab/acme/saas/pocs/my-repo',
        noteType: 'repository',
        repository: 'my-repo',
      };

      const result = classifyStar(repoMocNode, NOW);
      expect(result.classification).toBe('BLUE_GIANT');
      expect(result.badge).toBe('☀️ CLUSTER CORE');
      expect(result.label).toBe('Cluster Core (α Star)');
      expect(result.size).toBe(2.2);
      expect(result.spectralClass).toBe('α-CORE');
    });

    it('classifies Nebula Core for organizational group index nodes', () => {
      const groupMocNode = {
        id: 'sources/gitlab/acme/saas/pocs/index.md',
        category: 'sources/gitlab/acme/saas/pocs',
        noteType: 'group',
      };

      const result = classifyStar(groupMocNode, NOW);
      expect(result.classification).toBe('WHITE_CYAN');
      expect(result.badge).toBe('🌌 NEBULA CORE');
      expect(result.label).toBe('Nebula Core (MOC)');
      expect(result.size).toBe(1.8);
      expect(result.spectralClass).toBe('NEB-CORE');
    });
  });

  describe('STELLAR_SPECTRUM_LEGEND', () => {
    it('contains all 8 spectral classes in thermal order', () => {
      expect(STELLAR_SPECTRUM_LEGEND).toHaveLength(8);
      expect(STELLAR_SPECTRUM_LEGEND[0].classification).toBe('QUASAR');
      expect(STELLAR_SPECTRUM_LEGEND[1].classification).toBe('PULSAR');
      expect(STELLAR_SPECTRUM_LEGEND[2].classification).toBe('BLUE_GIANT');
      expect(STELLAR_SPECTRUM_LEGEND[7].classification).toBe('BROWN_DWARF');
    });
  });
});
