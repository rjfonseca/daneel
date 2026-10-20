export async function loadLatestNote(
  requestId: { current: number },
  fetchContent: () => Promise<string>,
  onContent: (content: string) => void,
  onError: () => void,
): Promise<void> {
  const current = ++requestId.current;
  try {
    const content = await fetchContent();
    if (requestId.current === current) onContent(content);
  } catch {
    if (requestId.current === current) onError();
  }
}
