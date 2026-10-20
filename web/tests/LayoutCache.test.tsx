import { describe, it, expect } from 'vitest';
import { saveLayout, loadLayout, CACHE_KEY } from '../src/lib/layoutCache';

describe('Feature: Otimização de Load (Cache de Coordenadas)', () => {
  it('Should save and retrieve node positions from localStorage', () => {
    const mockPositions = { 'node1': { x: 10, y: 20 }, 'node2': { x: -5, y: 15 } };
    
    // Test Saving
    saveLayout(mockPositions);
    
    const stored = localStorage.getItem(CACHE_KEY);
    expect(stored).not.toBeNull();
    expect(JSON.parse(stored!)).toEqual(mockPositions);
    
    // Test Loading
    const loaded = loadLayout();
    expect(loaded).toEqual(mockPositions);
  });
  
  it('Should return null if no layout is cached', () => {
    localStorage.clear();
    expect(loadLayout()).toBeNull();
  });
});
