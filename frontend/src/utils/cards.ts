export function getCardTitle(text: string): string {
  const firstLine = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find((line) => line.length > 0) ?? '';

  return firstLine
    .replace(/```.*/, '')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .trim()
    .slice(0, 100) + (firstLine.length > 100 ? '...' : '');
}

export function normalizeCardTitle(text: string): string {
  return getCardTitle(text)
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();
}
