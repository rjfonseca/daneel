export function getSearchShortcut(platform: string): { label: string; badge: string } {
  const isApple = /Mac|iPhone|iPad|iPod/i.test(platform);
  return isApple ? { label: 'Cmd+K', badge: '⌘' } : { label: 'Ctrl+K', badge: 'Ctrl' };
}
