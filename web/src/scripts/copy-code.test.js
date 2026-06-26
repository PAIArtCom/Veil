import { describe, it, expect } from 'vitest';
import { stripTrailingNewline } from './copy-code.js';

describe('stripTrailingNewline', () => {
  it('removes a single trailing newline', () => {
    expect(stripTrailingNewline('foo\n')).toBe('foo');
  });

  it('leaves text without a trailing newline unchanged', () => {
    expect(stripTrailingNewline('foo')).toBe('foo');
  });

  it('removes only one trailing newline when multiple are present', () => {
    expect(stripTrailingNewline('foo\n\n')).toBe('foo\n');
  });

  it('handles empty string', () => {
    expect(stripTrailingNewline('')).toBe('');
  });

  it('handles a string that is only a newline', () => {
    expect(stripTrailingNewline('\n')).toBe('');
  });

  it('preserves internal newlines', () => {
    expect(stripTrailingNewline('line1\nline2\n')).toBe('line1\nline2');
  });
});
