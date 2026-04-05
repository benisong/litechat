import React, { useState, useRef, useEffect } from 'react'
import { Send, Loader2 } from 'lucide-react'
import clsx from 'clsx'

export default function ChatInput({ onSend, disabled }) {
  const [text, setText] = useState('')
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

  return (
    <div className="glass border-t border-surface-border px-4 py-3
                    pb-[calc(env(safe-area-inset-bottom)+0.75rem)]">
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
