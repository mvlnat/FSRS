export function preserveNumericIdentifiers(markdown: string): string {
  const nextExpectedByIndent = new Map<number, number>();
  let activeFence: { marker: '`' | '~'; length: number } | null = null;

  return markdown
    .split(/(\r?\n)/)
    .map((segment) => {
      if (segment === '\n' || segment === '\r\n') {
        return segment;
      }

      const fenceMatch = /^ {0,3}(`{3,}|~{3,})/.exec(segment);
      if (fenceMatch) {
        const marker = fenceMatch[1][0] as '`' | '~';
        const length = fenceMatch[1].length;

        if (activeFence) {
          if (marker === activeFence.marker && length >= activeFence.length) {
            activeFence = null;
          }
        } else {
          activeFence = { marker, length };
        }

        return segment;
      }

      if (activeFence) {
        return segment;
      }

      const match = /^(\s{0,3})(\d+)([.)])(\s+)/.exec(segment);
      if (!match) {
        if (segment.trim()) {
          nextExpectedByIndent.clear();
        }
        return segment;
      }

      const [, indent, value, delimiter, spacing] = match;
      const markerNumber = Number(value);
      const indentLevel = indent.length;
      const expected = nextExpectedByIndent.get(indentLevel);
      const isConsecutiveListItem = markerNumber === 1 || markerNumber === expected;

      if (isConsecutiveListItem) {
        nextExpectedByIndent.set(indentLevel, markerNumber + 1);
        return segment;
      }

      nextExpectedByIndent.delete(indentLevel);
      return `${indent}${value}\\${delimiter}${spacing}${segment.slice(match[0].length)}`;
    })
    .join('');
}
