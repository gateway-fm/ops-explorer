import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { SourceCodeViewer } from './SourceCodeViewer';

const raw = '// File: contracts/Lib.sol\nlibrary Lib {}\n\n// File: contracts/Token.sol\ncontract Token {}\n\n';

describe('SourceCodeViewer', () => {
  const writeText = vi.fn();
  beforeEach(() => {
    writeText.mockReset();
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
  });

  it('copies the selected split file by default', () => {
    render(<SourceCodeViewer sourceCode={raw} contractName="Token" />);
    fireEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(writeText).toHaveBeenCalledWith('contract Token {}\n');
  });

  it('keeps the full stored source reachable and copyable when split into files', () => {
    render(<SourceCodeViewer sourceCode={raw} contractName="Token" />);
    fireEvent.click(screen.getByTestId('source-full'));
    expect(screen.getByTestId('source-full')).toHaveAttribute('aria-current', 'true');
    fireEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(writeText).toHaveBeenCalledWith(raw);
  });
});
