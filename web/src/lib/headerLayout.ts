export type HeaderDisclosure = 'more' | 'recent' | 'saved' | 'stats' | 'view' | null;
export type HeaderLayout = 'full' | 'condensed' | 'compact';

export function resolveHeaderLayout(width: number): HeaderLayout {
  if (width >= 1280) return 'full';
  if (width >= 768) return 'condensed';
  return 'compact';
}
