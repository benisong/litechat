export const STATUS_BAR_MARKER = '【状态栏】'

const FENCE_LINE_RE = /^[ \t]*(?:```|''')[^\n]*\n?/gm
const TRAILING_FENCE_RE = /[ \t]*(?:```|''')[^\n]*\n?$/g

export function splitStatusBar(content) {
  if (typeof content !== 'string') return { body: content, statusBar: '' }

  const markerIndex = content.lastIndexOf(STATUS_BAR_MARKER)
  if (markerIndex === -1) return { body: content, statusBar: '' }

  const lineBreakIndex = content.lastIndexOf('\n', markerIndex - 1)
  const panelStart = lineBreakIndex === -1 ? 0 : lineBreakIndex + 1
  const body = content.slice(0, panelStart)
  const statusBar = content.slice(panelStart).replace(FENCE_LINE_RE, '').trim()

  if (!statusBar) return { body: content, statusBar: '' }
  return {
    body: body.replace(TRAILING_FENCE_RE, '').trimEnd(),
    statusBar,
  }
}

// Normalize both the new separate status_bar field and legacy combined replies.
// The streamed raw reply is a final fallback for the just-completed assistant turn.
export function normalizeChatMessages(messages, latestStreamContent = '') {
  if (!Array.isArray(messages)) return []

  const latestAssistantIndex = [...messages].map(message => message.role).lastIndexOf('assistant')
  const streamed = splitStatusBar(latestStreamContent)

  return messages.map((message, index) => {
    if (message?.role !== 'assistant') return message

    const embedded = splitStatusBar(message.content)
    const persistedStatusBar = typeof message.status_bar === 'string'
      ? message.status_bar.trim()
      : ''
    const canUseStreamFallback = index === latestAssistantIndex
      && streamed.statusBar
      && String(message.content || '').trim() === String(streamed.body || '').trim()
    const statusBar = persistedStatusBar
      || embedded.statusBar
      || (canUseStreamFallback ? streamed.statusBar : '')

    if (!statusBar && !embedded.statusBar) return message
    return {
      ...message,
      content: embedded.statusBar ? embedded.body : message.content,
      status_bar: statusBar,
    }
  })
}
