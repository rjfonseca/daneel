/**
 * Utility to process and resolve Obsidian Wikilinks and relative Markdown links
 * against the galaxy node set.
 */

export interface GalaxyNodeRef {
  id: string;
  title?: string;
}

/**
 * Preprocesses markdown text by converting Obsidian wikilinks:
 * - `[[Note Title]]` -> `[Note Title](wikilink:Note%20Title)`
 * - `[[Target Note|Display Text]]` -> `[Display Text](wikilink:Target%20Note)`
 */
export function preprocessWikiLinks(markdown: string | null | undefined): string {
  if (!markdown) return '';
  
  // Match [[target]] or [[target|alias]]
  return markdown.replace(/\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g, (_match, target, alias) => {
    const trimmedTarget = target.trim();
    const display = alias ? alias.trim() : trimmedTarget;
    return `[${display}](wikilink:${encodeURIComponent(trimmedTarget)})`;
  });
}

/**
 * Normalizes a relative path given the current note directory.
 * E.g.
 * - currentDir: "notes/arquitetura", relative: "../../sources/reunioes/note.md" -> "sources/reunioes/note.md"
 * - currentDir: "notes", relative: "./ia/agentes.md" -> "notes/ia/agentes.md"
 */
export function normalizeRelativePath(currentDir: string, relativePath: string): string {
  const cleanRelative = relativePath.split('#')[0].split('?')[0];
  const combined = `${currentDir}/${cleanRelative}`;
  const parts = combined.split('/').filter(p => p && p !== '.');
  const stack: string[] = [];

  for (const part of parts) {
    if (part === '..') {
      if (stack.length > 0 && stack[stack.length - 1] !== '..') {
        stack.pop();
      } else {
        stack.push('..');
      }
    } else {
      stack.push(part);
    }
  }

  return stack.join('/');
}

/**
 * Resolves a clicked link href to a valid note ID or target path.
 * Supports:
 * 1. Wikilinks: `wikilink:Note%20Name`
 * 2. Relative markdown paths: `../../sources/note.md`
 * 3. Exact IDs or basename matches in availableNodes
 */
export function resolveInternalLink(
  href: string,
  currentNotePath?: string,
  availableNodes: (string | GalaxyNodeRef)[] = []
): string | null {
  if (!href) return null;

  // Ignore external links
  if (href.startsWith('http://') || href.startsWith('https://') || href.startsWith('mailto:')) {
    return null;
  }

  let rawTarget = href;
  if (href.startsWith('wikilink:')) {
    try {
      rawTarget = decodeURIComponent(href.replace(/^wikilink:/, ''));
    } catch {
      rawTarget = href.replace(/^wikilink:/, '');
    }
  }

  // Remove trailing anchor/query
  rawTarget = rawTarget.split('#')[0].split('?')[0].trim();
  if (!rawTarget) return null;

  // Normalize node list for fast lookup
  const nodes = availableNodes.map(n => typeof n === 'string' ? { id: n, title: n } : n);

  // 1. Check exact ID match
  const exactMatch = nodes.find(n => n.id === rawTarget || n.id === `${rawTarget}.md`);
  if (exactMatch) return exactMatch.id;

  // 2. If current note path is provided, resolve relative path
  if (currentNotePath) {
    const currentDir = currentNotePath.includes('/')
      ? currentNotePath.substring(0, currentNotePath.lastIndexOf('/'))
      : '';
    const normalized = normalizeRelativePath(currentDir, rawTarget);
    const relativeMatch = nodes.find(n => n.id === normalized || n.id === `${normalized}.md`);
    if (relativeMatch) return relativeMatch.id;
  }

  // 3. Match by basename (filename without extension or path)
  const targetBase = rawTarget.split('/').pop()?.replace(/\.md$/, '').toLowerCase();
  if (targetBase) {
    const baseMatch = nodes.find(n => {
      const nodeBase = n.id.split('/').pop()?.replace(/\.md$/, '').toLowerCase();
      if (nodeBase === targetBase) return true;
      if (n.title && n.title.toLowerCase() === targetBase) return true;
      return false;
    });
    if (baseMatch) return baseMatch.id;
  }

  // 4. Fallback to clean normalized path with .md extension if not present
  const fallback = rawTarget.endsWith('.md') ? rawTarget : `${rawTarget}.md`;
  return fallback;
}
