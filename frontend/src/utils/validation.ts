/**
 * Validate that a redirect path is internal (same-origin relative path).
 * Rejects absolute URLs and protocol-relative URLs.
 */
export function isInternalPath(path: string): boolean {
  if (typeof path !== 'string') return false
  if (!path.startsWith('/')) return false
  if (path.startsWith('//')) return false
  // Reject any string that looks like an absolute URL with a scheme
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(path)) return false
  return true
}
