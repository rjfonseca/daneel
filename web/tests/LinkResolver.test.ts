import { describe, it, expect } from 'vitest';
import {
  preprocessWikiLinks,
  normalizeRelativePath,
  resolveInternalLink
} from '../src/lib/linkResolver';

describe('LinkResolver', () => {
  describe('preprocessWikiLinks', () => {
    it('converts standard Obsidian wikilinks to custom markdown links', () => {
      const input = 'Conforme visto em [[Arquitetura Hexagonal]] e [[Domain Events]].';
      const output = preprocessWikiLinks(input);
      expect(output).toBe('Conforme visto em [Arquitetura Hexagonal](wikilink:Arquitetura%20Hexagonal) e [Domain Events](wikilink:Domain%20Events).');
    });

    it('converts wikilinks with alias to markdown links with display text', () => {
      const input = 'Veja [[notas/ia/agentes|Agentes Autônomos]] para detalhes.';
      const output = preprocessWikiLinks(input);
      expect(output).toBe('Veja [Agentes Autônomos](wikilink:notas%2Fia%2Fagentes) para detalhes.');
    });

    it('handles empty or null strings gracefully', () => {
      expect(preprocessWikiLinks('')).toBe('');
      expect(preprocessWikiLinks(null)).toBe('');
    });
  });

  describe('normalizeRelativePath', () => {
    it('normalizes parent traversal properly', () => {
      expect(normalizeRelativePath('notes/arquitetura', '../../sources/reunioes/2026.md'))
        .toBe('sources/reunioes/2026.md');
      expect(normalizeRelativePath('notes/ia/agentes', '../prompts.md'))
        .toBe('notes/ia/prompts.md');
      expect(normalizeRelativePath('notes', './carreira/senior.md'))
        .toBe('notes/carreira/senior.md');
    });
  });

  describe('resolveInternalLink', () => {
    const mockNodes = [
      { id: 'notes/arquitetura/render-rules-vs-presenter.md', title: 'Render Rules vs Presenter' },
      { id: 'sources/reunioes/2026-09-08-casa-de-frontend.md', title: 'Casa de Frontend' },
      { id: 'notes/ia/agentes.md', title: 'Agentes Autônomos' },
      { id: 'notes/carreira/plano-2026.md', title: 'Plano 2026' }
    ];

    it('ignores external URLs', () => {
      expect(resolveInternalLink('https://google.com')).toBeNull();
      expect(resolveInternalLink('http://example.com')).toBeNull();
      expect(resolveInternalLink('mailto:test@example.com')).toBeNull();
    });

    it('resolves exact match by node ID', () => {
      const resolved = resolveInternalLink('notes/ia/agentes.md', undefined, mockNodes);
      expect(resolved).toBe('notes/ia/agentes.md');
    });

    it('resolves relative markdown link', () => {
      const currentNote = 'notes/arquitetura/render-rules-vs-presenter.md';
      const href = '../../sources/reunioes/2026-09-08-casa-de-frontend.md';
      const resolved = resolveInternalLink(href, currentNote, mockNodes);
      expect(resolved).toBe('sources/reunioes/2026-09-08-casa-de-frontend.md');
    });

    it('resolves wikilink by title or basename', () => {
      const wikilink = 'wikilink:Agentes%20Aut%C3%B4nomos';
      const resolved = resolveInternalLink(wikilink, undefined, mockNodes);
      expect(resolved).toBe('notes/ia/agentes.md');
    });

    it('resolves wikilink by direct filename without .md', () => {
      const wikilink = 'wikilink:plano-2026';
      const resolved = resolveInternalLink(wikilink, undefined, mockNodes);
      expect(resolved).toBe('notes/carreira/plano-2026.md');
    });
  });
});
