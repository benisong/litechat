import React, { useState, useRef, useEffect } from 'react'
import { Send, Loader2 } from 'lucide-react'
import clsx from 'clsx'

const INPUT_TOOLS = [
  { key: 'colon', label: '：', open: '', close: '' },
  { key: 'quote', label: '\u201C\u201D', open: '\u201C', close: '\u201D' },
  { key: 'paren', label: '\uFF08\uFF09', open: '\uFF08', close: '\uFF09' },
]

export default function ChatInput({ onSend, disabled }) {
  const [text, setText] = useState('')
  const [activeTool, setActiveTool] = useState('colon')
  const textareaRef = useRef(null)

  // 自动调整高度
  useEffect(() => {
    const ta = textareaRef.current
    if (!ta) return
    ta.style.height = 'auto'
    ta.style.height = Math.min(ta.scrollHeight, 160) + 'px'
  }, [text])

  const handleSend = () => {
    const content = text.trim()
    if (!content || disabled) return
    onSend(content)
    setText('')
    setActiveTool('colon')
    // 重置高度
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
    }
  }

  const handleKeyDown = (e) => {
    // Enter 发送，Shift+Enter 换行
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  // 点击工具按钮
  const handleToolClick = (tool) => {
    setActiveTool(tool.key)
    const ta = textareaRef.current
    if (!ta) return

    if (tool.key === 'colon') {
      // 冒号：光标移到文本末尾
      ta.focus()
      const len = text.length
      ta.setSelectionRange(len, len)
      return
    }

    // 引号/括号：在光标位置插入符号对，光标放中间
    const start = ta.selectionStart
    const end = ta.selectionEnd
    const before = text.slice(0, start)
    const selected = text.slice(start, end)
    const after = text.slice(end)
    const newText = before + tool.open + selected + tool.close + after
    setText(newText)

    // 需要在 state 更新后设置光标
    requestAnimationFrame(() => {
      ta.focus()
      const cursorPos = start + tool.open.length + selected.length
      ta.setSelectionRange(cursorPos, cursorPos)
    })
  }

  return (
    <div className="glass border-t border-surface-border px-4 py-3
                    pb-[calc(env(safe-area-inset-bottom,0px)+1rem)]">
      {/* 输入辅助按钮 */}
      <div className="flex items-center gap-2 mb-2 px-1">
        {INPUT_TOOLS.map(tool => (
          <button
            key={tool.key}
            onClick={() => handleToolClick(tool)}
            disabled={disabled}
            className={clsx(
              'px-3 py-1 rounded-lg text-xs font-mono transition-all duration-150',
              'border active:scale-95',
              activeTool === tool.key
                ? 'bg-primary-600/20 border-primary-500/50 text-primary-300'
                : 'bg-surface border-surface-border text-gray-500 hover:text-gray-300 hover:border-gray-500',
              disabled && 'opacity-50 cursor-not-allowed'
            )}
          >
            {tool.label}
          </button>
        ))}
        <span className="text-[10px] text-gray-600 ml-1">
          {activeTool === 'quote' ? '对白' : activeTool === 'paren' ? '内心' : '叙述'}
        </span>
      </div>

      <div className="flex items-end gap-3">
        <textarea
          ref={textareaRef}
          value={text}
          onChange={e => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="发送消息…"
          rows={1}
          disabled={disabled}
          className={clsx(
            'flex-1 bg-surface border border-surface-border rounded-2xl',
            'px-4 py-3 text-sm text-white placeholder-gray-500',
            'outline-none focus:border-primary-500/60 focus:ring-1 focus:ring-primary-500/20',
            'resize-none transition-all duration-200',
            'max-h-40 overflow-y-auto',
            disabled && 'opacity-50 cursor-not-allowed'
          )}
        />

        <button
          onClick={handleSend}
          disabled={!text.trim() || disabled}
          className={clsx(
            'w-11 h-11 rounded-2xl flex items-center justify-center flex-shrink-0',
            'transition-all duration-150 active:scale-90',
            text.trim() && !disabled
              ? 'bg-primary-600 hover:bg-primary-500 text-white shadow-lg shadow-primary-600/30'
              : 'bg-surface border border-surface-border text-gray-600 cursor-not-allowed'
          )}
        >
          {disabled
            ? <Loader2 size={18} className="animate-spin" />
            : <Send size={18} className={text.trim() ? '' : 'opacity-50'} />
          }
        </button>
      </div>
    </div>
  )
}
