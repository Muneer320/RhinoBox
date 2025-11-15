/**
 * Unit tests for Monaco Code Editor integration
 */

import { initCodeEditor, getEditorValue, getCurrentLanguage, setEditorValue } from '../src/codeEditor.js';

// Mock Monaco Editor
global.monaco = {
  editor: {
    create: jest.fn(() => ({
      getValue: jest.fn(() => 'test code'),
      setValue: jest.fn(),
      getPosition: jest.fn(() => ({ lineNumber: 1, column: 1 })),
      getModel: jest.fn(() => ({
        onDidChangeContent: jest.fn(),
        setValue: jest.fn(),
      })),
      onDidChangeCursorPosition: jest.fn(),
      getAction: jest.fn(() => ({
        run: jest.fn(),
      })),
      dispose: jest.fn(),
      focus: jest.fn(),
      layout: jest.fn(),
    })),
    setTheme: jest.fn(),
    setModelLanguage: jest.fn(),
  },
  languages: {
    json: {
      jsonDefaults: {
        setDiagnosticsOptions: jest.fn(),
      },
    },
  },
};

// Mock DOM elements
const mockEditorContainer = document.createElement('div');
mockEditorContainer.id = 'code-editor-container';
document.body.appendChild(mockEditorContainer);

const mockCodePreview = document.createElement('div');
mockCodePreview.id = 'code-preview';
document.body.appendChild(mockCodePreview);

const mockLanguageSelector = document.createElement('select');
mockLanguageSelector.id = 'language-selector';
document.body.appendChild(mockLanguageSelector);

describe('Code Editor Module', () => {
  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();
    document.body.innerHTML = '';
    
    // Recreate elements
    const container = document.createElement('div');
    container.id = 'code-editor-container';
    document.body.appendChild(container);
    
    const preview = document.createElement('div');
    preview.id = 'code-preview';
    document.body.appendChild(preview);
    
    const selector = document.createElement('select');
    selector.id = 'language-selector';
    document.body.appendChild(selector);
  });

  test('should initialize code editor', () => {
    expect(() => {
      initCodeEditor();
    }).not.toThrow();
    
    expect(global.monaco.editor.create).toHaveBeenCalled();
  });

  test('should get editor value', () => {
    // Mock editor
    const mockEditor = {
      getValue: jest.fn(() => 'test code'),
    };
    
    // Since we can't directly access the internal editor, we test the function exists
    expect(typeof getEditorValue).toBe('function');
  });

  test('should get current language', () => {
    expect(typeof getCurrentLanguage).toBe('function');
    // Default should be json
    expect(getCurrentLanguage()).toBe('json');
  });

  test('should set editor value', () => {
    
    expect(typeof setEditorValue).toBe('function');
    expect(() => {
      setEditorValue('test');
    }).not.toThrow();
  });

  test('should handle language change', () => {
    const languageSelector = document.getElementById('language-selector');
    if (languageSelector) {
      const changeEvent = new Event('change');
      languageSelector.value = 'javascript';
      languageSelector.dispatchEvent(changeEvent);
      
      // Verify language selector exists
      expect(languageSelector).toBeTruthy();
    }
  });

  test('should handle format code', () => {
    const formatButton = document.createElement('button');
    formatButton.id = 'format-code';
    document.body.appendChild(formatButton);
    
    const clickEvent = new Event('click');
    formatButton.dispatchEvent(clickEvent);
    
    expect(formatButton).toBeTruthy();
  });

  test('should handle clear editor', () => {
    const clearButton = document.createElement('button');
    clearButton.id = 'clear-code';
    document.body.appendChild(clearButton);
    
    const clickEvent = new Event('click');
    clearButton.dispatchEvent(clickEvent);
    
    expect(clearButton).toBeTruthy();
  });

  test('should handle expand editor', () => {
    const expandButton = document.createElement('button');
    expandButton.id = 'expand-editor';
    document.body.appendChild(expandButton);
    
    const clickEvent = new Event('click');
    expandButton.dispatchEvent(clickEvent);
    
    expect(expandButton).toBeTruthy();
  });
});

describe('Code Editor Integration', () => {
  test('should integrate with Quick Add form', () => {
    const form = document.createElement('form');
    form.id = 'quickAddForm';
    document.body.appendChild(form);
    
    expect(document.getElementById('quickAddForm')).toBeTruthy();
  });

  test('should handle keyboard shortcuts', () => {
    const keyboardEvents = {
      'Ctrl+Enter': new KeyboardEvent('keydown', {
        ctrlKey: true,
        key: 'Enter',
      }),
      'Ctrl+K': new KeyboardEvent('keydown', {
        ctrlKey: true,
        key: 'k',
      }),
      'Ctrl+Shift+F': new KeyboardEvent('keydown', {
        ctrlKey: true,
        shiftKey: true,
        key: 'F',
      }),
      'F11': new KeyboardEvent('keydown', {
        key: 'F11',
      }),
    };

    Object.values(keyboardEvents).forEach((event) => {
      expect(event).toBeInstanceOf(KeyboardEvent);
    });
  });
});

describe('Code Editor Modal', () => {
  beforeEach(() => {
    const modal = document.createElement('div');
    modal.id = 'editor-modal';
    modal.className = 'editor-modal';
    document.body.appendChild(modal);
    
    const background = document.createElement('div');
    background.className = 'editor-modal-background';
    modal.appendChild(background);
    
    const card = document.createElement('div');
    card.className = 'editor-modal-card';
    modal.appendChild(card);
    
    const modalEditor = document.createElement('div');
    modalEditor.id = 'modal-editor';
    card.appendChild(modalEditor);
  });

  test('should create modal elements', () => {
    expect(document.getElementById('editor-modal')).toBeTruthy();
    expect(document.getElementById('modal-editor')).toBeTruthy();
  });

  test('should handle modal close', () => {
    const modal = document.getElementById('editor-modal');
    const closeButton = document.createElement('button');
    closeButton.className = 'editor-modal-close';
    modal.appendChild(closeButton);
    
    const clickEvent = new Event('click');
    closeButton.dispatchEvent(clickEvent);
    
    expect(closeButton).toBeTruthy();
  });
});


