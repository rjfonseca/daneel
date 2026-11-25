import { describe, it, expect } from 'vitest';
import {
  sanitizeArmsStationLabel,
  getSafeHudBounds,
  clampOutwardCalloutToSafeBounds,
  shouldShowOverviewRootNebula,
  doesRectOverlapBoxes,
} from '../src/lib/galaxy/labelLayout';
import { calculateOutwardCalloutGeometry, getOutwardCalloutBoundingBox } from '../src/lib/canvasMath';

describe('labelLayout', () => {
  it('sanitizes redundant ARMS station prefixes and suffixes concisely', () => {
    expect(sanitizeArmsStationLabel('Skill: Meetings Processor', 'skills', true)).toBe('SKILL · Meetings Proce..');
    expect(sanitizeArmsStationLabel('Skill: Meetings Processor', 'skills', false)).toBe('SKILL · Meetings Processor');
    expect(sanitizeArmsStationLabel('GitLab Harvester', 'harvesters', true)).toBe('HARVESTER · GitLab');
    expect(sanitizeArmsStationLabel('Routine: Dream Cycle', 'routines', true)).toBe('ROUTINE · Dream Cycle');
  });

  it('caps overview root nebulas to a concise top budget while revealing all on zoom or focus', () => {
    // At overview (ratio >= 0.55) with maxNebulas = 6 -> cap at 4
    expect(shouldShowOverviewRootNebula(0, 6, 0.9, false)).toBe(true);
    expect(shouldShowOverviewRootNebula(3, 6, 0.9, false)).toBe(true);
    expect(shouldShowOverviewRootNebula(4, 6, 0.9, false)).toBe(false);

    // Zoomed in or focused -> reveal all root nebulas
    expect(shouldShowOverviewRootNebula(10, 6, 0.45, false)).toBe(true);
    expect(shouldShowOverviewRootNebula(10, 6, 0.9, true)).toBe(true);
  });

  it('clamps outward ARMS callout boxes inside safe viewport bounds below TopCosmicBar and above Hotbar', () => {
    const safe = getSafeHudBounds(1920, 901, 111, 825);
    expect(safe.minY).toBeGreaterThanOrEqual(120);
    expect(safe.maxY).toBeLessThanOrEqual(815);

    // Station at top (12 o'clock) whose outward callout would clip at y ~ 20 behind TopCosmicBar
    const textWidth = 140;
    const rawGeom = calculateOutwardCalloutGeometry(960, 50, 960, 450, textWidth);
    const rawBox = getOutwardCalloutBoundingBox(rawGeom, textWidth);
    expect(rawBox.minY).toBeLessThan(safe.minY);

    const clamped = clampOutwardCalloutToSafeBounds(rawGeom, rawBox, safe);
    expect(clamped.box.minY).toBeGreaterThanOrEqual(safe.minY);
    expect(clamped.box.maxY).toBeLessThanOrEqual(safe.maxY);
    expect(clamped.box.minX).toBeGreaterThanOrEqual(safe.minX);
    expect(clamped.box.maxX).toBeLessThanOrEqual(safe.maxX);
  });

  it('detects AABB overlap against existing callouts with margin', () => {
    const existing = [{ minX: 100, maxX: 200, minY: 100, maxY: 140 }];
    expect(doesRectOverlapBoxes({ minX: 180, maxX: 240, minY: 120, maxY: 160 }, existing, 4)).toBe(true);
    expect(doesRectOverlapBoxes({ minX: 210, maxX: 260, minY: 100, maxY: 140 }, existing, 4)).toBe(false);
  });
});
