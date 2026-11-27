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

  it('falls back to the first file when the source changes while Full source is selected', () => {
    const { rerender } = render(<SourceCodeViewer sourceCode={raw} contractName="Token" />);
    fireEvent.click(screen.getByTestId('source-full'));
    rerender(<SourceCodeViewer sourceCode="contract Other {}" contractName="Other" />);
    expect(screen.getByTestId('source-code')).toHaveTextContent('contract Other {}');
    fireEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(writeText).toHaveBeenCalledWith('contract Other {}');
  });

  it('offers the stored standard JSON when it holds a single source', () => {
    const json = JSON.stringify({
      language: 'Solidity',
      sources: { 'Token.sol': { content: 'contract Token {}' } },
      settings: { optimizer: { enabled: true, runs: 200 } },
    });
    render(<SourceCodeViewer sourceCode={json} contractName="Token" />);
    fireEvent.click(screen.getByTestId('source-full'));
    fireEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(writeText).toHaveBeenCalledWith(json);
  });

  it('has no Full source entry when the stored text is the one file shown', () => {
    render(<SourceCodeViewer sourceCode="contract Token {}" contractName="Token" />);
    expect(screen.queryByTestId('source-full')).not.toBeInTheDocument();
  });

  it('keeps the full stored source reachable and copyable when split into files', () => {
    render(<SourceCodeViewer sourceCode={raw} contractName="Token" />);
    fireEvent.click(screen.getByTestId('source-full'));
    expect(screen.getByTestId('source-full')).toHaveAttribute('aria-current', 'true');
    fireEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(writeText).toHaveBeenCalledWith(raw);
  });
});
