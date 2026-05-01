import { describe, expect, it } from 'vitest';
import { preserveNumericIdentifiers } from './markdown';

describe('preserveNumericIdentifiers', () => {
  it('escapes nonconsecutive numeric markers so problem IDs render literally', () => {
    expect(
      preserveNumericIdentifiers([
        'Segment Tree',
        '',
        '406. Queue Reconstruction By Height',
        '699. Falling Squares',
        '1649. Create Sorted Array Through Instructions',
      ].join('\n')),
    ).toBe([
      'Segment Tree',
      '',
      '406\\. Queue Reconstruction By Height',
      '699\\. Falling Squares',
      '1649\\. Create Sorted Array Through Instructions',
    ].join('\n'));
  });

  it('leaves normal consecutive ordered lists unchanged', () => {
    expect(
      preserveNumericIdentifiers([
        'Steps',
        '',
        '1. Read the question',
        '2. Solve it',
        '3. Review the answer',
      ].join('\n')),
    ).toBe([
      'Steps',
      '',
      '1. Read the question',
      '2. Solve it',
      '3. Review the answer',
    ].join('\n'));
  });

  it('does not alter numeric markers inside fenced code blocks', () => {
    expect(
      preserveNumericIdentifiers([
        '```text',
        '406. Queue Reconstruction By Height',
        '699. Falling Squares',
        '```',
      ].join('\n')),
    ).toBe([
      '```text',
      '406. Queue Reconstruction By Height',
      '699. Falling Squares',
      '```',
    ].join('\n'));
  });
});
