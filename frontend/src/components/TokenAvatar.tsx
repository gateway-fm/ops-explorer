// Curated avatar palette in OKLCH. Hues are non-semantic (no greens / reds
// that could be misread as success / error) and all sit at similar visual
// weight (L≈0.62–0.70) so no token "shouts" relative to the others. White
// foreground reads correctly on every entry in both light and dark themes.
const AVATAR_COLORS = [
  'oklch(0.62 0.21 286)', // purple (brand)
  'oklch(0.62 0.20 250)', // indigo
  'oklch(0.70 0.15 230)', // sky
  'oklch(0.70 0.15 195)', // teal
  'oklch(0.68 0.20 50)',  // orange
  'oklch(0.65 0.21 350)', // magenta
] as const;

function colorFor(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = ((h << 5) - h + seed.charCodeAt(i)) | 0;
  }
  return AVATAR_COLORS[Math.abs(h) % AVATAR_COLORS.length];
}

type Size = 'sm' | 'md' | 'lg';

const SIZES: Record<Size, string> = {
  sm: 'h-7 w-7 text-xs',
  md: 'h-10 w-10 text-base',
  lg: 'h-14 w-14 text-xl',
};

export function TokenAvatar({
  symbol,
  address,
  size = 'md',
}: {
  symbol: string;
  address: string;
  size?: Size;
}) {
  const initial =
    (symbol || '').replace(/[^A-Za-z0-9]/g, '').charAt(0).toUpperCase() || '?';
  return (
    <div
      className={`flex shrink-0 items-center justify-center rounded-full font-semibold leading-none text-white shadow-[inset_0_-2px_0_rgba(0,0,0,0.08),0_1px_2px_rgba(0,0,0,0.08)] ${SIZES[size]}`}
      style={{ backgroundColor: colorFor(address.toLowerCase()) }}
      aria-hidden
    >
      {initial}
    </div>
  );
}
