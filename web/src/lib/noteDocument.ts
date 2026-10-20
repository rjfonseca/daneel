import { parseDocument as parseYamlDocument } from 'yaml';
import { unified } from 'unified';
import remarkGfm from 'remark-gfm';
import remarkParse from 'remark-parse';

const MAX_FRONTMATTER_SIZE = 64 * 1024;

export interface NoteDocument {
  body: string;
  metadata: Record<string, unknown> | null;
  hasMalformedFrontmatter: boolean;
}

export interface NoteHeading {
  id: string;
  text: string;
  level: number;
}

interface MarkdownNode {
  type: string;
  depth?: number;
  value?: string;
  alt?: string;
  children?: MarkdownNode[];
  position?: {
    start: { offset?: number };
    end: { offset?: number };
  };
}

const markdownParser = unified().use(remarkParse).use(remarkGfm);

function parseMarkdown(markdown: string): MarkdownNode {
  return markdownParser.parse(markdown) as unknown as MarkdownNode;
}

function headingText(node: MarkdownNode): string {
  if (node.type === 'image') return node.alt || '';
  if (node.value) return node.value;
  return node.children?.map(headingText).join('') || '';
}

function headingSlug(text: string): string {
  return text.toLocaleLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .trim()
    .replace(/\s+/g, '-') || 'section';
}

export function parseNoteDocument(markdown: string): NoteDocument {
  const openingFence = markdown.match(/^---(?:\r?\n)/);
  if (!openingFence) {
    return { body: markdown, metadata: null, hasMalformedFrontmatter: false };
  }

  const contentStart = openingFence[0].length;
  const closingFence = /^---[ \t]*\r?$/gm;
  closingFence.lastIndex = contentStart;
  const closingMatch = closingFence.exec(markdown);
  if (!closingMatch) {
    return { body: markdown, metadata: null, hasMalformedFrontmatter: true };
  }

  try {
    if (closingMatch.index > MAX_FRONTMATTER_SIZE) {
      return { body: markdown, metadata: null, hasMalformedFrontmatter: true };
    }

    const source = markdown.slice(contentStart, closingMatch.index);
    const parsed = parseYamlDocument(source, {
      version: '1.2',
      schema: 'core',
      resolveKnownTags: false,
      uniqueKeys: true,
    });
    if (parsed.errors.length > 0 || parsed.warnings.length > 0) {
      return { body: markdown, metadata: null, hasMalformedFrontmatter: true };
    }

    const value: unknown = parsed.toJS({ maxAliasCount: 100 });
    const metadata = value && typeof value === 'object' && !Array.isArray(value)
      ? value as Record<string, unknown>
      : null;
    if (!metadata) return { body: markdown, metadata: null, hasMalformedFrontmatter: true };

    const bodyStart = closingMatch.index + closingMatch[0].length;
    return {
      body: markdown.slice(bodyStart).replace(/^\r?\n/, ''),
      metadata,
      hasMalformedFrontmatter: false,
    };
  } catch {
    return { body: markdown, metadata: null, hasMalformedFrontmatter: true };
  }
}

export function removeDuplicateTitleHeading(markdown: string, title: string): string {
  const normalizedTitle = title.trim().replace(/\s+/g, ' ').toLocaleLowerCase();
  if (!normalizedTitle) return markdown;
  const firstBlock = parseMarkdown(markdown).children?.[0] as MarkdownNode | undefined;
  if (firstBlock?.type !== 'heading' || firstBlock.depth !== 1) return markdown;
  if (headingText(firstBlock).trim().replace(/\s+/g, ' ').toLocaleLowerCase() !== normalizedTitle) return markdown;

  const start = firstBlock.position?.start.offset;
  const end = firstBlock.position?.end.offset;
  if (start === undefined || end === undefined) return markdown;
  const lineStart = markdown.lastIndexOf('\n', start - 1) + 1;
  const nextLine = markdown.indexOf('\n', end);
  const lineEnd = nextLine === -1 ? markdown.length : nextLine + 1;
  return markdown.slice(0, lineStart) + markdown.slice(lineEnd);
}

export function getNoteHeadings(markdown: string, title = ''): NoteHeading[] {
  const source = removeDuplicateTitleHeading(markdown, title);
  const headings: NoteHeading[] = [];
  const slugCounts = new Map<string, number>();
  const visit = (node: MarkdownNode) => {
    if (node.type === 'heading') {
      const text = headingText(node).trim();
      const baseId = headingSlug(text);
      const count = slugCounts.get(baseId) ?? 0;
      slugCounts.set(baseId, count + 1);
      headings.push({ id: count ? `${baseId}-${count}` : baseId, text, level: node.depth || 1 });
    }
    node.children?.forEach(visit);
  };

  parseMarkdown(source).children?.forEach(visit);
  return headings;
}
