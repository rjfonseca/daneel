import { describe, expect, it } from 'vitest';
import { getNoteHeadings, parseNoteDocument, removeDuplicateTitleHeading } from '../src/lib/noteDocument';

describe('note document helpers', () => {
  it('extracts YAML frontmatter while preserving the Markdown body', () => {
    const document = parseNoteDocument('---\ntitle: Ada\ntags:\n  - math\n---\n# Notes\nBody');

    expect(document.metadata).toEqual({ title: 'Ada', tags: ['math'] });
    expect(document.body).toBe('# Notes\nBody');
    expect(document.hasMalformedFrontmatter).toBe(false);
  });

  it('keeps malformed frontmatter visible as Markdown content', () => {
    const markdown = '---\ntitle: [broken\n---\n# Still visible';

    expect(parseNoteDocument(markdown)).toEqual({
      body: markdown,
      metadata: null,
      hasMalformedFrontmatter: true,
    });
  });

  it('preserves oversized or executable-tag frontmatter without evaluating it', () => {
    const oversized = `---\nvalue: ${'x'.repeat(66 * 1024)}\n---\nBody`;
    const executableTag = '---\nvalue: !!js/function >\n  function () { return 1 }\n---\nBody';

    expect(parseNoteDocument(oversized)).toEqual({
      body: oversized,
      metadata: null,
      hasMalformedFrontmatter: true,
    });
    expect(parseNoteDocument(executableTag)).toEqual({
      body: executableTag,
      metadata: null,
      hasMalformedFrontmatter: true,
    });
  });

  it('omits a first H1 that duplicates the reader title and creates stable heading ids', () => {
    expect(getNoteHeadings('# Ada Lovelace\n## Early life\n### Education', 'Ada Lovelace')).toEqual([
      { id: 'early-life', text: 'Early life', level: 2 },
      { id: 'education', text: 'Education', level: 3 },
    ]);
  });

  it('only removes a matching title when it is the leading Markdown heading', () => {
    const markdown = '# Overview\n\n# Ada\n\nDetails';
    expect(removeDuplicateTitleHeading(markdown, 'Ada')).toBe(markdown);
    expect(getNoteHeadings(markdown, 'Ada')).toEqual([
      { id: 'overview', text: 'Overview', level: 1 },
      { id: 'ada', text: 'Ada', level: 1 },
    ]);
    expect(removeDuplicateTitleHeading('~~~md\n# Ada\n~~~', 'Ada')).toBe('~~~md\n# Ada\n~~~');
  });

  it('inventories headings nested in blockquotes and lists using Markdown structure', () => {
    expect(getNoteHeadings('> ## Quoted\n\n- ## Listed\n\n## Ordinary')).toEqual([
      { id: 'quoted', text: 'Quoted', level: 2 },
      { id: 'listed', text: 'Listed', level: 2 },
      { id: 'ordinary', text: 'Ordinary', level: 2 },
    ]);
  });

  it('tracks rendered setext headings and ignores both fenced code styles', () => {
    const markdown = 'First\n=====\n\n~~~md\n# Hidden\n~~~\n\nSecond\n------\n\n```md\n# Also hidden\n```\n\n## Third';
    expect(getNoteHeadings(markdown)).toEqual([
      { id: 'first', text: 'First', level: 1 },
      { id: 'second', text: 'Second', level: 2 },
      { id: 'third', text: 'Third', level: 2 },
    ]);
  });
});
