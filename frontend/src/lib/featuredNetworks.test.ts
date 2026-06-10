import { describe, it, expect } from 'vitest';
import { isHttpUrl } from './featuredNetworks';

describe('isHttpUrl', () => {
  it('accepts http and https URLs', () => {
    expect(isHttpUrl('http://localhost:3001')).toBe(true);
    expect(isHttpUrl('https://explorer.example.com')).toBe(true);
    expect(isHttpUrl('https://explorer.example.com/path?q=1')).toBe(true);
  });

  it('rejects non-http(s) schemes from the operator-controlled list', () => {
    expect(isHttpUrl('javascript:alert(1)')).toBe(false);
    expect(isHttpUrl('data:text/html,<script>alert(1)</script>')).toBe(false);
    expect(isHttpUrl('ftp://example.com')).toBe(false);
    expect(isHttpUrl('file:///etc/passwd')).toBe(false);
  });

  it('rejects values that are not absolute URLs', () => {
    expect(isHttpUrl('/relative/path')).toBe(false);
    expect(isHttpUrl('not a url')).toBe(false);
    expect(isHttpUrl('')).toBe(false);
  });
});
