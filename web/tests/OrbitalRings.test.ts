import { describe, it, expect } from 'vitest';
import { calculateOrbitalLayout } from '../src/lib/orbitalRings';

describe('OrbitalRings (ARMS Polar Geometry)', () => {
  it('deve calcular o centro e distribuir satélites no Perímetro Seldon unificado', () => {
    const mockNodes = [
      { id: 'n1', ring: 'memory', x: 100, y: 100 },
      { id: 'n2', ring: 'memory', x: -100, y: -100 },
      { id: 'skill-1', ring: 'skills' },
      { id: 'skill-2', ring: 'skills' },
      { id: 'routine-1', ring: 'routines' },
      { id: 'harvester-1', ring: 'harvesters' },
    ];

    const layout = calculateOrbitalLayout(mockNodes, { baseMargin: 200, defaultRadius: 500 });
    expect(layout.center.x).toBe(0);
    expect(layout.center.y).toBe(0);
    expect(layout.rings).toHaveLength(1);
    expect(layout.rings[0].id).toBe('perimeter');

    // Satélites receberam posições no raio do perímetro
    const skillPos = layout.nodePositions.get('skill-1');
    expect(skillPos).toBeDefined();
    if (skillPos) {
      const dist = Math.hypot(skillPos.x - layout.center.x, skillPos.y - layout.center.y);
      expect(Math.round(dist)).toBe(Math.round(layout.rings[0].radius));
    }

    const harvesterPos = layout.nodePositions.get('harvester-1');
    expect(harvesterPos).toBeDefined();
    if (harvesterPos) {
      const dist = Math.hypot(harvesterPos.x - layout.center.x, harvesterPos.y - layout.center.y);
      expect(Math.round(dist)).toBe(Math.round(layout.rings[0].radius));
    }
  });

  it('deve distribuir nós do mesmo anel com espaçamento polar equidistante', () => {
    const mockNodes = [
      { id: 'n1', ring: 'memory', x: 0, y: 0 },
      { id: 's1', ring: 'skills' },
      { id: 's2', ring: 'skills' },
      { id: 's3', ring: 'skills' },
      { id: 's4', ring: 'skills' },
    ];

    const layout = calculateOrbitalLayout(mockNodes, { baseMargin: 100, defaultRadius: 400 });
    const s1 = layout.nodePositions.get('s1')!;
    const s2 = layout.nodePositions.get('s2')!;
    const s3 = layout.nodePositions.get('s3')!;


    // Ângulo entre 4 nós equidistantes deve ser pi/2 (90 graus)
    const angleDiff12 = Math.abs(s2.angle - s1.angle);
    const angleDiff23 = Math.abs(s3.angle - s2.angle);
    expect(angleDiff12).toBeCloseTo(Math.PI / 2, 4);
    expect(angleDiff23).toBeCloseTo(Math.PI / 2, 4);
  });
});
