/**
 * Removes a single trailing newline from a string.
 * Clipboard pastes from <pre><code> often include a trailing newline
 * that causes an unwanted extra line when pasting into a terminal.
 *
 * @param {string} text
 * @returns {string}
 */
export function stripTrailingNewline(text) {
  return text.replace(/\n$/, '');
}
