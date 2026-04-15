export function formatTokenValue(value: string | number | undefined, decimals = 18): string {
  const strValue = String(value ?? '0');
  if (!strValue || strValue === '0') return '0';
  try {
    const num = BigInt(strValue);
    const divisor = BigInt(10) ** BigInt(decimals);
    const wholePart = num / divisor;
    const fracPart = num % divisor;
    if (fracPart === 0n) {
      return wholePart.toLocaleString();
    }
    const fracStr = fracPart.toString().padStart(decimals, '0');
    const trimmed = fracStr.slice(0, 4).replace(/0+$/, '');
    if (trimmed) {
      return `${wholePart.toLocaleString()}.${trimmed}`;
    }
    return wholePart.toLocaleString();
  } catch {
    return strValue;
  }
}
