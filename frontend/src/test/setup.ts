import '@testing-library/jest-dom/vitest';
import { afterAll, afterEach, beforeAll } from 'vitest';
import { server } from './msw';

// MSW node server lifecycle (PLAN §6). Declarative fetch mocking for RTL tests
// that don't vi.mock('../lib/api'). `onUnhandledRequest: 'bypass'` lets
// vi.mock-based tests (which never hit the network) coexist without warnings;
// MSW-based tests assert behaviour explicitly.
beforeAll(() => server.listen({ onUnhandledRequest: 'bypass' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
