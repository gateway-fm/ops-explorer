import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { ThemeProvider, useTheme } from './useTheme';

// The theme toggle source of truth (PLAN §6): ThemeProvider syncs theme to
// <html class="dark"> + localStorage; useTheme.setTheme drives both.

function Probe() {
  const { theme, setTheme } = useTheme();
  return (
    <div>
      <span data-testid="theme">{theme}</span>
      <button onClick={() => setTheme('dark')}>dark</button>
      <button onClick={() => setTheme('light')}>light</button>
    </div>
  );
}

describe('ThemeProvider / useTheme', () => {
  afterEach(() => vi.restoreAllMocks());
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove('dark');
  });

  it('defaults to light and does not set the dark class', () => {
    render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(screen.getByTestId('theme')).toHaveTextContent('light');
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('setTheme(dark) toggles <html class="dark"> and persists to localStorage', () => {
    render(<ThemeProvider><Probe /></ThemeProvider>);
    act(() => screen.getByText('dark').click());
    expect(document.documentElement.classList.contains('dark')).toBe(true);
    expect(localStorage.getItem('theme')).toBe('dark');
  });

  it('reverting to light removes the class and updates storage', () => {
    render(<ThemeProvider><Probe /></ThemeProvider>);
    act(() => screen.getByText('dark').click());
    act(() => screen.getByText('light').click());
    expect(document.documentElement.classList.contains('dark')).toBe(false);
    expect(localStorage.getItem('theme')).toBe('light');
  });

  it('reads the persisted theme on mount', () => {
    localStorage.setItem('theme', 'dark');
    render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(screen.getByTestId('theme')).toHaveTextContent('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
  it('keeps rendering and switching in memory when persistence is denied', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError'); });
    render(<ThemeProvider><Probe/></ThemeProvider>);
    expect(screen.getByTestId('theme')).toHaveTextContent('light');
    act(() => screen.getByText('dark').click()); expect(document.documentElement.classList.contains('dark')).toBe(true);
    act(() => screen.getByText('light').click()); expect(document.documentElement.classList.contains('dark')).toBe(false);
  });
});
