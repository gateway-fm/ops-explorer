const quantity = /^(0|[1-9][0-9]{0,77})$/;

export function exactQuantity(value: string | null | undefined): string {
  if (value === null) return 'Hidden';
  if (value === undefined) return 'Unavailable';
  if (!quantity.test(value)) throw Error('Invalid quantity');
  return BigInt(value).toLocaleString('en-US');
}

export function exactUnits(value: string | null | undefined, decimals = 18): string {
  if (value === null) return 'Hidden';
  if (value === undefined) return 'Unavailable';
  if (!quantity.test(value) || !Number.isSafeInteger(decimals) || decimals < 0 || decimals > 255) throw Error('Invalid quantity');
  if (!decimals) return exactQuantity(value);
  const padded = value.padStart(decimals + 1, '0');
  const whole = padded.slice(0, -decimals), fraction = padded.slice(-decimals).replace(/0+$/, '');
  return BigInt(whole).toLocaleString('en-US') + (fraction ? `.${fraction}` : '');
}

export function exactTimestamp(value: string | undefined): string {
  if (value === undefined) return 'Unavailable';
  if (!quantity.test(value)) throw Error('Invalid timestamp');
  const stamp = BigInt(value);
  return stamp <= 8640000000000n ? new Date(Number(stamp) * 1000).toISOString().replace('T', ' ').replace('.000Z', ' UTC') : `${value} Unix seconds`;
}

// Returned fiat prices are decimal strings. Multiplication never passes through
// floating point, including very small balances and prices.
export function exactFiatValue(balance: string, decimals: number, price: string): string {
  if (!quantity.test(balance) || !Number.isSafeInteger(decimals) || decimals < 0 || decimals > 255 || !/^(0|[1-9][0-9]*)(\.[0-9]+)?$/.test(price) || price.length > 512) throw Error('Invalid price');
  const [whole, fraction = ''] = price.split('.');
  const value = BigInt(balance) * BigInt(whole + fraction), scale = decimals + fraction.length;
  const padded = value.toString().padStart(scale + 1, '0');
  const head = scale ? padded.slice(0, -scale) : padded, tail = scale ? padded.slice(-scale).replace(/0+$/, '') : '';
  return BigInt(head).toLocaleString('en-US') + (tail ? `.${tail}` : '');
}
