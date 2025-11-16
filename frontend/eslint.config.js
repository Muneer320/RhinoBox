import js from '@eslint/js'
import globals from 'globals'

export default [
  {
    ignores: ['dist/**', 'node_modules/**'],
  },
  {
    files: ['**/*.{js,mjs}'],
    languageOptions: {
      ecmaVersion: 'latest',
      globals: globals.browser,
      sourceType: 'module',
    },
    rules: {
      ...js.configs.recommended.rules,
      'no-unused-vars': ['error', { varsIgnorePattern: '^[A-Z_]', argsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }],
      'no-console': ['warn', { allow: ['warn', 'error'] }],
    },
  },
  {
    files: ['*.config.js', 'test-connection.js', 'tests/**/*.{js,mjs}'],
    languageOptions: { globals: globals.node },
  },
  {
    files: ['tests/**/*.{js,mjs}'],
    languageOptions: {
      globals: Object.fromEntries([
        'describe', 'it', 'test', 'expect', 'beforeEach', 'afterEach',
        'beforeAll', 'afterAll', 'vi',
      ].map(name => [name, 'readonly'])),
    },
  },
]
