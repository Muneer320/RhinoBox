// Monaco Editor Integration for RhinoBox
import * as monaco from 'monaco-editor';

let codeEditor = null;
let modalEditor = null;
let editorModal = null;
let currentLanguage = 'json';
let isFullscreen = false;

// Language mapping for Monaco
const LANGUAGE_MAP = {
  json: 'json',
  javascript: 'javascript',
  typescript: 'typescript',
  python: 'python',
  java: 'java',
  go: 'go',
  cpp: 'cpp',
  csharp: 'csharp',
  ruby: 'ruby',
  php: 'php',
  sql: 'sql',
  html: 'html',
  css: 'css',
  yaml: 'yaml',
  xml: 'xml',
  markdown: 'markdown',
  text: 'plaintext',
  url: 'plaintext',
};

// Get Monaco theme based on current app theme
function getMonacoTheme() {
  const theme = document.documentElement.getAttribute('data-theme') || 'light';
  return theme === 'dark' ? 'vs-dark' : 'vs-light';
}

// Initialize the compact code editor
export function initCodeEditor() {
  const editorContainer = document.getElementById('code-editor-container');
  if (!editorContainer) {
    console.warn('Code editor container not found');
    return;
  }

  // Initialize Monaco Editor
  codeEditor = monaco.editor.create(editorContainer, {
    value: '{\n  "example": "data"\n}',
    language: currentLanguage,
    theme: getMonacoTheme(),
    automaticLayout: true,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    fontSize: 14,
    lineNumbers: 'on',
    roundedSelection: true,
    bracketPairColorization: { enabled: true },
    suggest: {
      snippetsPreventQuickSuggestions: false,
    },
    quickSuggestions: true,
    folding: true,
    foldingStrategy: 'indentation',
    wordWrap: 'off',
    tabSize: 2,
    insertSpaces: true,
    formatOnPaste: true,
    formatOnType: true,
  });

  // Update status bar
  updateStatusBar(codeEditor, 'editor');

  // Listen to editor changes
  codeEditor.onDidChangeCursorPosition(() => {
    updateStatusBar(codeEditor, 'editor');
  });

  codeEditor.getModel().onDidChangeContent(() => {
    updateStatusBar(codeEditor, 'editor');
    updateEditorSize(codeEditor, 'editor');
  });

  // Setup click to expand
  const codePreview = document.getElementById('code-preview');
  if (codePreview) {
    codePreview.addEventListener('click', openEditorModal);
    codePreview.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        openEditorModal();
      }
    });
  }

  // Setup language selector
  const languageSelector = document.getElementById('language-selector');
  if (languageSelector) {
    languageSelector.addEventListener('change', (e) => {
      changeLanguage(e.target.value);
    });
  }

  // Setup format button
  const formatButton = document.getElementById('format-code');
  if (formatButton) {
    formatButton.addEventListener('click', formatCode);
  }

  // Setup expand button
  const expandButton = document.getElementById('expand-editor');
  if (expandButton) {
    expandButton.addEventListener('click', openEditorModal);
  }

  // Setup clear button
  const clearButton = document.getElementById('clear-code');
  if (clearButton) {
    clearButton.addEventListener('click', clearEditor);
  }

  // Setup keyboard shortcuts
  setupKeyboardShortcuts();

  // Listen for theme changes
  const observer = new MutationObserver(() => {
    if (codeEditor) {
      monaco.editor.setTheme(getMonacoTheme());
    }
    if (modalEditor) {
      monaco.editor.setTheme(getMonacoTheme());
    }
  });

  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme'],
  });

  // Setup JSON validation
  if (currentLanguage === 'json') {
    monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
      validate: true,
      schemas: [],
      allowComments: false,
    });
  }
}

// Open editor modal
export function openEditorModal() {
  editorModal = document.getElementById('editor-modal');
  const modalEditorContainer = document.getElementById('modal-editor');

  if (!editorModal || !modalEditorContainer) {
    console.warn('Editor modal elements not found');
    return;
  }

  // Create modal editor instance
  const currentValue = codeEditor ? codeEditor.getValue() : '';

  modalEditor = monaco.editor.create(modalEditorContainer, {
    value: currentValue,
    language: LANGUAGE_MAP[currentLanguage] || currentLanguage,
    theme: getMonacoTheme(),
    automaticLayout: true,
    minimap: { enabled: true },
    fontSize: 16,
    lineNumbers: 'on',
    scrollBeyondLastLine: false,
    bracketPairColorization: { enabled: true },
    suggest: {
      snippetsPreventQuickSuggestions: false,
    },
    quickSuggestions: true,
    folding: true,
    foldingStrategy: 'indentation',
    wordWrap: 'off',
    tabSize: 2,
    insertSpaces: true,
    formatOnPaste: true,
    formatOnType: true,
  });

  // Update status bar for modal
  updateStatusBar(modalEditor, 'modal');
  modalEditor.onDidChangeCursorPosition(() => {
    updateStatusBar(modalEditor, 'modal');
  });

  modalEditor.getModel().onDidChangeContent(() => {
    updateStatusBar(modalEditor, 'modal');
    updateEditorSize(modalEditor, 'modal');
  });

  // Sync language selector
  const modalLanguageSelector = document.getElementById('modal-language-selector');
  if (modalLanguageSelector) {
    modalLanguageSelector.value = currentLanguage;
    modalLanguageSelector.addEventListener('change', (e) => {
      changeLanguage(e.target.value, true);
    });
  }

  // Setup modal buttons
  const modalFormatButton = document.getElementById('modal-format');
  if (modalFormatButton) {
    modalFormatButton.addEventListener('click', () => formatCode(true));
  }

  const copyButton = document.getElementById('copy-code');
  if (copyButton) {
    copyButton.addEventListener('click', copyToClipboard);
  }

  const modalSubmitButton = document.getElementById('modal-submit');
  if (modalSubmitButton) {
    modalSubmitButton.addEventListener('click', submitFromModal);
  }

  // Setup close handlers
  const closeButtons = editorModal.querySelectorAll('.editor-modal-close');
  closeButtons.forEach((btn) => {
    btn.addEventListener('click', closeEditorModal);
  });

  const modalBackground = editorModal.querySelector('.editor-modal-background');
  if (modalBackground) {
    modalBackground.addEventListener('click', closeEditorModal);
  }

  // ESC key to close
  const escapeHandler = (e) => {
    if (e.key === 'Escape' && editorModal.classList.contains('is-active')) {
      closeEditorModal();
      document.removeEventListener('keydown', escapeHandler);
    }
  };
  document.addEventListener('keydown', escapeHandler);
  editorModal._escapeHandler = escapeHandler;

  // Show modal
  editorModal.classList.add('is-active');
  document.body.style.overflow = 'hidden';
  modalEditor.focus();

  // Fullscreen support
  setupFullscreen();
}

// Close editor modal
function closeEditorModal() {
  if (!editorModal || !modalEditor) return;

  // Sync content back to main editor
  if (codeEditor) {
    codeEditor.setValue(modalEditor.getValue());
  }

  // Dispose modal editor
  modalEditor.dispose();
  modalEditor = null;

  // Remove escape handler
  if (editorModal._escapeHandler) {
    document.removeEventListener('keydown', editorModal._escapeHandler);
    delete editorModal._escapeHandler;
  }

  // Hide modal
  editorModal.classList.remove('is-active');
  document.body.style.overflow = '';

  // Exit fullscreen if active
  if (isFullscreen) {
    exitFullscreen();
  }
}

// Change language
function changeLanguage(language, isModal = false) {
  currentLanguage = language;
  const monacoLanguage = LANGUAGE_MAP[language] || 'plaintext';

  if (codeEditor) {
    monaco.editor.setModelLanguage(codeEditor.getModel(), monacoLanguage);
  }

  if (modalEditor) {
    monaco.editor.setModelLanguage(modalEditor.getModel(), monacoLanguage);
  }

  // Update language display
  const languageDisplay = document.getElementById('editor-language-display');
  if (languageDisplay) {
    languageDisplay.textContent = language.toUpperCase();
  }

  const modalLanguageDisplay = document.getElementById('modal-editor-language');
  if (modalLanguageDisplay) {
    modalLanguageDisplay.textContent = language.toUpperCase();
  }

  // Sync language selectors
  const languageSelector = document.getElementById('language-selector');
  if (languageSelector && !isModal) {
    languageSelector.value = language;
  }

  const modalLanguageSelector = document.getElementById('modal-language-selector');
  if (modalLanguageSelector && isModal) {
    modalLanguageSelector.value = language;
  }

  // Update validation for JSON
  if (language === 'json') {
    monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
      validate: true,
      schemas: [],
      allowComments: false,
    });
  }
}

// Format code
function formatCode(isModal = false) {
  const editor = isModal ? modalEditor : codeEditor;
  if (!editor) return;

  try {
    editor.getAction('editor.action.formatDocument').run();
  } catch (error) {
    console.warn('Format action not available:', error);
  }
}

// Clear editor
function clearEditor() {
  if (codeEditor) {
    codeEditor.setValue('');
  }
}

// Copy to clipboard
async function copyToClipboard() {
  const editor = modalEditor || codeEditor;
  if (!editor) return;

  const value = editor.getValue();
  try {
    await navigator.clipboard.writeText(value);
    // Show toast notification (assuming showToast is available globally)
    if (window.showToast) {
      window.showToast('Code copied to clipboard');
    }
  } catch (error) {
    console.error('Failed to copy to clipboard:', error);
    // Fallback
    const textArea = document.createElement('textarea');
    textArea.value = value;
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand('copy');
    document.body.removeChild(textArea);
    if (window.showToast) {
      window.showToast('Code copied to clipboard');
    }
  }
}

// Update status bar
function updateStatusBar(editor, type) {
  if (!editor) return;

  const position = editor.getPosition();
  const lineCol = `Ln ${position.lineNumber}, Col ${position.column}`;

  if (type === 'editor') {
    const lineColElement = document.getElementById('editor-line-col');
    if (lineColElement) {
      lineColElement.textContent = lineCol;
    }
  } else if (type === 'modal') {
    const lineColElement = document.getElementById('modal-editor-line-col');
    if (lineColElement) {
      lineColElement.textContent = lineCol;
    }
  }
}

// Update editor size
function updateEditorSize(editor, type) {
  if (!editor) return;

  const value = editor.getValue();
  const size = new Blob([value]).size;
  const sizeText = formatBytes(size);

  if (type === 'editor') {
    const sizeElement = document.getElementById('editor-size');
    if (sizeElement) {
      sizeElement.textContent = sizeText;
    }
  } else if (type === 'modal') {
    const sizeElement = document.getElementById('modal-editor-size');
    if (sizeElement) {
      sizeElement.textContent = sizeText;
    }
  }
}

// Format bytes
function formatBytes(bytes) {
  if (bytes === 0) return '0 bytes';
  const k = 1024;
  const sizes = ['bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Setup keyboard shortcuts
function setupKeyboardShortcuts() {
  document.addEventListener('keydown', (e) => {
    // Ctrl+Enter - Submit
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      submitCode();
    }

    // Ctrl+K - Clear (only if not in input)
    if ((e.ctrlKey || e.metaKey) && e.key === 'k' && !e.shiftKey) {
      const target = e.target;
      if (
        target.tagName !== 'INPUT' &&
        target.tagName !== 'TEXTAREA' &&
        !target.isContentEditable
      ) {
        e.preventDefault();
        clearEditor();
      }
    }

    // Ctrl+Shift+F - Format
    if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'F') {
      const target = e.target;
      if (
        target.tagName !== 'INPUT' &&
        target.tagName !== 'TEXTAREA' &&
        !target.isContentEditable
      ) {
        e.preventDefault();
        formatCode();
      }
    }

    // F11 - Toggle fullscreen
    if (e.key === 'F11') {
      e.preventDefault();
      if (editorModal && editorModal.classList.contains('is-active')) {
        toggleFullscreen();
      }
    }
  });
}

// Setup fullscreen
function setupFullscreen() {
  const fullscreenButton = document.getElementById('expand-editor');
  if (fullscreenButton) {
    fullscreenButton.addEventListener('click', toggleFullscreen);
  }
}

// Toggle fullscreen
function toggleFullscreen() {
  if (!editorModal) return;

  if (!isFullscreen) {
    editorModal.classList.add('is-fullscreen');
    isFullscreen = true;
    if (modalEditor) {
      setTimeout(() => {
        modalEditor.layout();
      }, 100);
    }
  } else {
    exitFullscreen();
  }
}

// Exit fullscreen
function exitFullscreen() {
  if (editorModal) {
    editorModal.classList.remove('is-fullscreen');
    isFullscreen = false;
    if (modalEditor) {
      setTimeout(() => {
        modalEditor.layout();
      }, 100);
    }
  }
}

// Submit code from main editor
export function submitCode() {
  if (!codeEditor) return;

  const code = codeEditor.getValue();
  const language = currentLanguage;

  // Trigger form submission
  const form = document.getElementById('quickAddForm');
  if (form) {
    // Set the value in a hidden input or trigger the existing form handler
    const event = new Event('submit', { bubbles: true, cancelable: true });
    form.dispatchEvent(event);
  }
}

// Submit code from modal
function submitFromModal() {
  if (!modalEditor) return;

  // Sync to main editor first
  if (codeEditor) {
    codeEditor.setValue(modalEditor.getValue());
  }

  // Close modal
  closeEditorModal();

  // Submit
  submitCode();
}

// Get editor value
export function getEditorValue() {
  const editor = modalEditor || codeEditor;
  return editor ? editor.getValue() : '';
}

// Get current language
export function getCurrentLanguage() {
  return currentLanguage;
}

// Set editor value
export function setEditorValue(value) {
  if (codeEditor) {
    codeEditor.setValue(value || '');
  }
}

// Dispose editors
export function disposeEditors() {
  if (codeEditor) {
    codeEditor.dispose();
    codeEditor = null;
  }
  if (modalEditor) {
    modalEditor.dispose();
    modalEditor = null;
  }
}

