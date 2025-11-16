import { vi } from 'vitest'
Object.defineProperty(window, 'matchMedia', { configurable: true, value: vi.fn(query => ({
  media: query, matches: false, onchange: null,
  addListener: vi.fn(), removeListener: vi.fn(),
  addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn(),
})) })
