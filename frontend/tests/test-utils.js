/**
 * Test utilities for unit testing
 * Provides basic test framework functionality
 */

// Simple test framework implementation
export const describe = (name, fn) => {
  console.group(`\n📦 ${name}`)
  try {
    fn()
  } catch (error) {
    console.error(`❌ Test suite "${name}" failed:`, error)
  }
  console.groupEnd()
}

export const it = (name, fn) => {
  try {
    fn()
    console.log(`  ✅ ${name}`)
  } catch (error) {
    console.error(`  ❌ ${name}:`, error)
    throw error
  }
}

export const expect = (actual) => {
  return {
    toBe: (expected) => {
      if (actual !== expected) {
        throw new Error(`Expected ${expected}, but got ${actual}`)
      }
    },
    toBeTruthy: () => {
      if (!actual) {
        throw new Error(`Expected truthy value, but got ${actual}`)
      }
    },
    toBeFalsy: () => {
      if (actual) {
        throw new Error(`Expected falsy value, but got ${actual}`)
      }
    },
    toContain: (substring) => {
      if (typeof actual !== 'string' || !actual.includes(substring)) {
        throw new Error(`Expected "${actual}" to contain "${substring}"`)
      }
    },
    toBeInstanceOf: (constructor) => {
      if (!(actual instanceof constructor)) {
        throw new Error(`Expected instance of ${constructor.name}, but got ${typeof actual}`)
      }
    }
  }
}

export const beforeEach = (fn) => {
  if (typeof fn === 'function') {
    fn()
  }
}

export const afterEach = (fn) => {
  if (typeof fn === 'function') {
    fn()
  }
}

