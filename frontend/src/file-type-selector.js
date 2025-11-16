let selectedType = 'auto'
const initialized = new WeakSet()

export function initFileTypeSelector(root = document, onChange = () => {}) {
  const container = root.querySelector('.type-buttons')
  if (!container || initialized.has(container)) return
  initialized.add(container)
  const buttons = Array.from(container.querySelectorAll('.type-btn'))
  function select(button, announce = true) {
    for (const item of buttons) {
      item.classList.toggle('active', item === button)
      item.setAttribute('aria-checked', String(item === button))
    }
    selectedType = button.dataset.type
    if (announce) onChange(selectedType)
  }
  for (const button of buttons) button.addEventListener('click', () => select(button))
  container.addEventListener('keydown', event => {
    const index = Math.max(0, buttons.findIndex(button => button.classList.contains('active')))
    let next = index
    if (['ArrowRight', 'ArrowDown'].includes(event.key)) next = (index + 1) % buttons.length
    else if (['ArrowLeft', 'ArrowUp'].includes(event.key)) next = (index - 1 + buttons.length) % buttons.length
    else if (['Enter', ' '].includes(event.key)) { event.preventDefault(); buttons[index].click(); return }
    else return
    event.preventDefault()
    select(buttons[next], false)
    buttons[next].focus()
  })
}

export function getSelectedFileType() { return selectedType }
export function resetFileTypeSelector() { document.querySelector('.type-btn[data-type="auto"]')?.click() }
