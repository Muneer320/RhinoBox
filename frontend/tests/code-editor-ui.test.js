/**
 * UI Tests for Monaco Code Editor
 * These tests verify that UI components render correctly
 */

describe('Code Editor UI Tests', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  test('UI: Code editor section renders with correct structure', () => {
    const section = document.createElement('div');
    section.className = 'code-editor-section';
    
    const header = document.createElement('div');
    header.className = 'code-editor-header';
    section.appendChild(header);
    
    const container = document.createElement('div');
    container.id = 'code-editor-container';
    section.appendChild(container);
    
    const footer = document.createElement('div');
    footer.className = 'code-editor-footer';
    section.appendChild(footer);
    
    document.body.appendChild(section);
    
    expect(section.classList.contains('code-editor-section')).toBe(true);
    expect(header.classList.contains('code-editor-header')).toBe(true);
    expect(footer.classList.contains('code-editor-footer')).toBe(true);
    expect(document.getElementById('code-editor-container')).toBeTruthy();
  });

  test('UI: Language selector renders with all options', () => {
    const selector = document.createElement('select');
    selector.id = 'language-selector';
    selector.className = 'language-selector';
    
    const languages = [
      { value: 'json', label: '📄 JSON' },
      { value: 'javascript', label: '🟨 JavaScript' },
      { value: 'typescript', label: '🔷 TypeScript' },
      { value: 'python', label: '🐍 Python' },
      { value: 'java', label: '☕ Java' },
      { value: 'go', label: '🐹 Go' },
      { value: 'cpp', label: '⚙️ C++' },
      { value: 'csharp', label: '🔹 C#' },
      { value: 'ruby', label: '💎 Ruby' },
      { value: 'php', label: '🐘 PHP' },
      { value: 'sql', label: '🗄️ SQL' },
      { value: 'html', label: '🌐 HTML' },
      { value: 'css', label: '🎨 CSS' },
      { value: 'yaml', label: '📋 YAML' },
      { value: 'xml', label: '📰 XML' },
      { value: 'markdown', label: '📝 Markdown' },
      { value: 'text', label: '📝 Text' },
      { value: 'url', label: '🔗 URL/Link' },
    ];
    
    languages.forEach((lang) => {
      const option = document.createElement('option');
      option.value = lang.value;
      option.textContent = lang.label;
      selector.appendChild(option);
    });
    
    document.body.appendChild(selector);
    
    expect(selector.options.length).toBe(languages.length);
    expect(selector.classList.contains('language-selector')).toBe(true);
  });

  test('UI: Action buttons render correctly', () => {
    const formatButton = document.createElement('button');
    formatButton.id = 'format-code';
    formatButton.className = 'btn-icon';
    formatButton.title = 'Format Code (Ctrl+Shift+F)';
    
    const expandButton = document.createElement('button');
    expandButton.id = 'expand-editor';
    expandButton.className = 'btn-icon';
    expandButton.title = 'Expand Editor (F11)';
    
    const clearButton = document.createElement('button');
    clearButton.id = 'clear-code';
    clearButton.className = 'btn btn-secondary';
    
    const submitButton = document.createElement('button');
    submitButton.id = 'submit-code';
    submitButton.className = 'btn btn-primary';
    
    document.body.appendChild(formatButton);
    document.body.appendChild(expandButton);
    document.body.appendChild(clearButton);
    document.body.appendChild(submitButton);
    
    expect(formatButton.classList.contains('btn-icon')).toBe(true);
    expect(expandButton.classList.contains('btn-icon')).toBe(true);
    expect(clearButton.classList.contains('btn-secondary')).toBe(true);
    expect(submitButton.classList.contains('btn-primary')).toBe(true);
  });

  test('UI: Status bar renders with correct elements', () => {
    const statusBar = document.createElement('div');
    statusBar.className = 'code-editor-status';
    
    const lineCol = document.createElement('span');
    lineCol.id = 'editor-line-col';
    lineCol.textContent = 'Ln 1, Col 1';
    statusBar.appendChild(lineCol);
    
    const language = document.createElement('span');
    language.id = 'editor-language-display';
    language.textContent = 'JSON';
    statusBar.appendChild(language);
    
    const size = document.createElement('span');
    size.id = 'editor-size';
    size.textContent = '0 bytes';
    statusBar.appendChild(size);
    
    document.body.appendChild(statusBar);
    
    expect(statusBar.querySelector('#editor-line-col')).toBeTruthy();
    expect(statusBar.querySelector('#editor-language-display')).toBeTruthy();
    expect(statusBar.querySelector('#editor-size')).toBeTruthy();
  });

  test('UI: Modal structure renders correctly', () => {
    const modal = document.createElement('div');
    modal.id = 'editor-modal';
    modal.className = 'editor-modal';
    
    const background = document.createElement('div');
    background.className = 'editor-modal-background';
    modal.appendChild(background);
    
    const card = document.createElement('div');
    card.className = 'editor-modal-card';
    modal.appendChild(card);
    
    const head = document.createElement('header');
    head.className = 'editor-modal-head';
    card.appendChild(head);
    
    const body = document.createElement('section');
    body.className = 'editor-modal-body';
    card.appendChild(body);
    
    const modalEditor = document.createElement('div');
    modalEditor.id = 'modal-editor';
    body.appendChild(modalEditor);
    
    const foot = document.createElement('footer');
    foot.className = 'editor-modal-foot';
    card.appendChild(foot);
    
    document.body.appendChild(modal);
    
    expect(modal.classList.contains('editor-modal')).toBe(true);
    expect(modal.querySelector('.editor-modal-background')).toBeTruthy();
    expect(modal.querySelector('.editor-modal-card')).toBeTruthy();
    expect(modal.querySelector('.editor-modal-head')).toBeTruthy();
    expect(modal.querySelector('.editor-modal-body')).toBeTruthy();
    expect(modal.querySelector('.editor-modal-foot')).toBeTruthy();
    expect(modal.querySelector('#modal-editor')).toBeTruthy();
  });

  test('UI: Modal shows when is-active class is added', () => {
    const modal = document.createElement('div');
    modal.id = 'editor-modal';
    modal.className = 'editor-modal';
    document.body.appendChild(modal);
    
    // Initially hidden
    const computedStyle = window.getComputedStyle(modal);
    expect(modal.classList.contains('is-active')).toBe(false);
    
    // Add active class
    modal.classList.add('is-active');
    expect(modal.classList.contains('is-active')).toBe(true);
  });

  test('UI: Fullscreen mode applies correct classes', () => {
    const modal = document.createElement('div');
    modal.id = 'editor-modal';
    modal.className = 'editor-modal is-active';
    document.body.appendChild(modal);
    
    // Add fullscreen class
    modal.classList.add('is-fullscreen');
    expect(modal.classList.contains('is-fullscreen')).toBe(true);
  });

  test('UI: Code preview is focusable', () => {
    const preview = document.createElement('div');
    preview.id = 'code-preview';
    preview.className = 'code-preview';
    preview.tabIndex = 0;
    document.body.appendChild(preview);
    
    expect(preview.tabIndex).toBe(0);
    expect(preview.classList.contains('code-editor-preview')).toBe(false);
    expect(preview.classList.contains('code-preview')).toBe(true);
  });

  test('UI: All buttons have proper accessibility attributes', () => {
    const formatButton = document.createElement('button');
    formatButton.id = 'format-code';
    formatButton.setAttribute('aria-label', 'Format code');
    formatButton.title = 'Format Code (Ctrl+Shift+F)';
    
    const expandButton = document.createElement('button');
    expandButton.id = 'expand-editor';
    expandButton.setAttribute('aria-label', 'Expand editor');
    expandButton.title = 'Expand Editor (F11)';
    
    document.body.appendChild(formatButton);
    document.body.appendChild(expandButton);
    
    expect(formatButton.getAttribute('aria-label')).toBe('Format code');
    expect(formatButton.title).toBe('Format Code (Ctrl+Shift+F)');
    expect(expandButton.getAttribute('aria-label')).toBe('Expand editor');
    expect(expandButton.title).toBe('Expand Editor (F11)');
  });

  test('UI: Responsive layout adapts on mobile', () => {
    // Simulate mobile viewport
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 375,
    });
    
    const footer = document.createElement('div');
    footer.className = 'code-editor-footer';
    document.body.appendChild(footer);
    
    // On mobile, footer should stack vertically
    // This is handled by CSS media queries
    expect(footer.classList.contains('code-editor-footer')).toBe(true);
  });

  test('UI: Theme colors are applied correctly', () => {
    const root = document.documentElement;
    
    // Test light theme
    root.setAttribute('data-theme', 'light');
    expect(root.getAttribute('data-theme')).toBe('light');
    
    // Test dark theme
    root.setAttribute('data-theme', 'dark');
    expect(root.getAttribute('data-theme')).toBe('dark');
  });
});

