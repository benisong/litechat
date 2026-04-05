import React, { useState } from 'react'
import clsx from 'clsx'
import Avatar from '../ui/Avatar'
import { Trash2, Copy, Check } from 'lucide-react'
import { useChatStore } from '../../store'

export default function MessageBubble({ message, character }) {
  const [copied, setCopied] = useState(false)
  const [showActions, setShowActions] = useState(false)
  const { deleteMessage } = useChatStore()

  const isUser = message.role === 'user'
  const isStreaming = message.isStreaming

  const handleCopy = async () => {
    await navigator.clipboard.writeText(message.content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleDelete = async () => {
    if (!message.id.startsWith('temp')) {
      await deleteMessage(message.id)
    }
  }

  return (
    <div
      className={clsx(
        'flex gap-2.5 px-4 message-enter',
        isUser ? 'flex-row-reverse' : 'flex-row'
      )}
      onLongPress={() => setShowActions(true)}
    >
      {/* 头像 */}
      {!isUser && (
        <Avatar
          name={character?.name}
          src={character?.avatar_url}
          size="sm"
          className="mt-0.5 flex-shrink-0"
        />
      )}

      {/* 气泡 */}
      <div className={clsx('max-w-[78%] group', isUser ? 'items-end' : 'items-start', 'flex flex-col gap-1')}>
        {/* 角色名 */}
        {!isUser && (
          <span className="text-xs text-gray-500 px-1">
            {character?.name || 'AI'}
          </span>
        )}

        <div className="relative">
          <div
            className={clsx(
              'text-sm leading-relaxed whitespace-pre-wrap break-words',
              isUser ? 'bubble-user' : 'bubble-ai',
              isStreaming && !message.content && 'min-w-[60px] min-h-[36px]'
            )}
            onClick={() => setShowActions(v => !v)}
          >
            {message.content || (isStreaming ? '' : '...')}
            {/* 流式打字光标 */}
            {isStreaming && <span className="typing-cursor" />}
          </div>

          {/* 操作按钮（点击显示） */}
          {showActions && !isStreaming && (
            <div className={clsx(
              'absolute top-0 flex gap-1 animate-fade-in',
              isUser ? 'right-full mr-2' : 'left-full ml-2'
            )}>
              <button
                onClick={handleCopy}
                className="p-1.5 rounded-lg bg-surface border border-surface-border
                           text-gray-400 hover:text-white transition-colors"
              >
                {copied ? <Check size={14} className="text-green-400" /> : <Copy size={14} />}
              </button>
              {!message.id.startsWith('temp') && (
                <button
                  onClick={handleDelete}
                  className="p-1.5 rounded-lg bg-surface border border-surface-border
                             text-gray-400 hover:text-red-400 transition-colors"
                >
                  <Trash2 size={14} />
                </button>
              )}
            </div>
          )}
        </div>

        {/* 时间戳 */}
        <span className="text-[10px] text-gray-600 px-1">
          {new Date(message.created_at).toLocaleTimeString('zh-CN', {
            hour: '2-digit', minute: '2-digit'
          })}
        </span>
      </div>
    </div>
  )
}
