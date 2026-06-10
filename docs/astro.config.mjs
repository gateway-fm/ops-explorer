// @ts-check
import { execSync } from 'node:child_process';
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Resolve the version these docs describe: the latest git tag, overridable via
// the DOCS_VERSION env var, with a fallback for environments where tags aren't
// available (e.g. a shallow CI checkout — the Pages workflow fetches tags).
function resolveDocsVersion() {
  if (process.env.DOCS_VERSION) return process.env.DOCS_VERSION;
  try {
    return execSync('git describe --tags --abbrev=0', { encoding: 'utf8' }).trim();
  } catch {
    return 'v0.8.2';
  }
}
const DOCS_VERSION = resolveDocsVersion();

// https://astro.build/config
export default defineConfig({
  vite: {
    define: {
      'import.meta.env.PUBLIC_DOCS_VERSION': JSON.stringify(DOCS_VERSION),
    },
  },
  // GitHub Pages project site: https://gateway-fm.github.io/block-explorer/
  site: 'https://gateway-fm.github.io',
  base: '/block-explorer',
  integrations: [
    starlight({
      title: 'Block Explorer',
      description:
        'Documentation for the Gateway block explorer — a lightweight, self-hosted explorer for EVM-compatible chains.',
      logo: {
        // The real explorer mark (frontend/public/logo.svg), shown beside the wordmark.
        src: './src/assets/logo.svg',
        replacesTitle: false,
      },
      // Load the explorer's exact fonts from the Google Fonts CDN (works on GitHub Pages).
      head: [
        {
          tag: 'link',
          attrs: { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'preconnect',
            href: 'https://fonts.gstatic.com',
            crossorigin: true,
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'stylesheet',
            href: 'https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap',
          },
        },
      ],
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/gateway-fm/block-explorer',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/gateway-fm/block-explorer/edit/main/docs/',
      },
      customCss: ['./src/styles/custom.css'],
      components: {
        // Adds a version badge next to the site title in the header.
        SiteTitle: './src/components/SiteTitle.astro',
      },
      // Branded code blocks: JetBrains Mono + calm GitHub syntax + purple active-tab
      // indicator. Surface backgrounds/borders are applied in custom.css at runtime
      // (EC processes styleOverride colours at build time, where CSS vars can't be
      // used and per-theme surfaces are brittle — plain CSS is the reliable layer).
      expressiveCode: {
        themes: ['github-dark-default', 'github-light-default'],
        styleOverrides: {
          borderRadius: '0.75rem',
          codeFontFamily: "'JetBrains Mono', Menlo, Monaco, Consolas, monospace",
          codeFontSize: '0.78rem',
          codeLineHeight: '1.55',
          frames: {
            editorActiveTabIndicatorTopColor: '#8950fa',
          },
        },
      },
      sidebar: [
        {
          label: 'Start Here',
          items: [
            { label: 'Introduction', slug: 'getting-started/introduction' },
            { label: 'Quickstart', slug: 'getting-started/quickstart' },
          ],
        },
        {
          label: 'Configuration',
          items: [
            { label: 'Overview', slug: 'configuration/overview' },
            { label: 'Branding & Whitelabel', slug: 'configuration/branding' },
            { label: 'Network Modes', slug: 'configuration/network-modes' },
            { label: 'Deployment Modes', slug: 'configuration/deployment-modes' },
          ],
        },
        {
          label: 'Privacy',
          items: [
            { label: 'Privacy Mode', slug: 'privacy/overview' },
            { label: 'How It Works', slug: 'privacy/how-it-works' },
            { label: 'Authentication & SSO', slug: 'privacy/authentication' },
            { label: 'View as User', slug: 'privacy/view-as-user' },
            { label: 'Deploying Privacy Mode', slug: 'privacy/deployment' },
          ],
        },
        {
          label: 'Core Features',
          items: [{ label: 'Feature Overview', slug: 'features/overview' }],
        },
        {
          label: 'Architecture',
          items: [{ label: 'System Architecture', slug: 'architecture/overview' }],
        },
        {
          label: 'API Reference',
          items: [{ label: 'Public REST API', slug: 'api/public-api' }],
        },
        {
          label: 'Design',
          items: [{ label: 'Style Guide', slug: 'design/style-guide' }],
        },
      ],
    }),
  ],
});
