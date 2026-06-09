import { describe, it, expect } from 'vitest';
import { shouldRetryQuery } from './queryRetry';

describe('shouldRetryQuery', () => {
  it('does not retry non-retryable 4xx (403/404)', () => {
    expect(shouldRetryQuery(0, new Error('API error: 403'))).toBe(false);
    expect(shouldRetryQuery(0, new Error('API error: 404'))).toBe(false);
    expect(shouldRetryQuery(2, new Error('API error: 400'))).toBe(false);
    expect(shouldRetryQuery(0, new Error('API error: 401'))).toBe(false);
  });

  it('retries the retryable 4xx (408/429) until the cap', () => {
    expect(shouldRetryQuery(0, new Error('API error: 408'))).toBe(true);
    expect(shouldRetryQuery(0, new Error('API error: 429'))).toBe(true);
    expect(shouldRetryQuery(2, new Error('API error: 429'))).toBe(true);
    // capped at 3 attempts
    expect(shouldRetryQuery(3, new Error('API error: 429'))).toBe(false);
  });

  it('retries 5xx until the cap', () => {
    expect(shouldRetryQuery(0, new Error('API error: 500'))).toBe(true);
    expect(shouldRetryQuery(2, new Error('API error: 503'))).toBe(true);
    expect(shouldRetryQuery(3, new Error('API error: 500'))).toBe(false);
  });

  it('retries network/parse errors with no status code until the cap', () => {
    expect(shouldRetryQuery(0, new Error('Failed to fetch'))).toBe(true);
    expect(shouldRetryQuery(2, new Error('NetworkError'))).toBe(true);
    expect(shouldRetryQuery(3, new Error('Failed to fetch'))).toBe(false);
  });

  it('handles prefixed error messages (detail + status)', () => {
    expect(shouldRetryQuery(0, new Error('contract not found: API error: 404'))).toBe(false);
    expect(shouldRetryQuery(0, new Error('upstream down: API error: 502'))).toBe(true);
  });

  it('handles non-Error throwables defensively (retries until cap)', () => {
    expect(shouldRetryQuery(0, 'string thrown' as unknown as Error)).toBe(true);
    expect(shouldRetryQuery(0, null as unknown as Error)).toBe(true);
    expect(shouldRetryQuery(3, undefined as unknown as Error)).toBe(false);
  });
});
