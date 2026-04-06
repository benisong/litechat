import React, { useState } from 'react'
import clsx from 'clsx'
import Avatar from '../ui/Avatar'
import MessageContent from './MessageContent'
import { Trash2, Copy, Check, RefreshCw } from 'lucide-react'
import { useChatStore } from '../../store'

export default function MessageBubble({ message, character, onRegenerate, onDeleteCascade }) {
  const [copied, setCopied] = useState(false)

  const isUser = message.role === 'user'
  const isStreaming = message.isStreaming
  const isTemp = message.id?.startsWith('temp')

  const handleCopy = async (e) => {
    e.stopPropagation()
    await navigator.clipboard.writeText(message.content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleDelete = async (e) => {
    e.stopPropagation()
    if (isTemp) return
    if (onDeleteCascade) {
      if (confirm('删除此消息及之后的所有消息？')) {
        onDeleteCascade(message.id)
      }
    }
  }

  return (
    <div
      className={clsx(
        'flex gap-2.5 px-4 message-enter',
        isUser ? 'flex-row-reverse' : 'flex-row'
      )}
    >
      {/* AI 头像 */}
      {!isUser && (
        <Avatar
          name={character?.name}
          src={character?.avatar_url}
          size="sm"
          className="mt-0.5 flex-shrink-0"
        />
      )}

      {/* 气泡区域 */}
      <div className={clsx('max-w-[78%] flex flex-col gap-1', isUser ? 'items-end' : 'items-start')}>
        {/* 角色名 */}
        {!isUser && (
          <span className="text-xs text-gray-500 px-1">
            {character?.name || 'AI'}
          </span>
        )}

        {/* 气泡内容 */}
        <div
          className={clsx(
            'text-sm leading-relaxed break-words',
            isUser ? 'bubble-user' : 'bubble-ai',
            isStreaming && !message.content && 'min-w-[60px] min-h-[36px]'
          )}
        >
          {isUser ? (
            // 用户消息：保留空白格式
            <span className="whitespace-pre-wrap">{message.content}</span>
          ) : (
            // AI 消息：渲染 markdown + 思考块
            <>
              <MessageContent content={message.content} isUser={false} />
              {isStreaming && <span className="typing-cursor" />}
            </>
          )}
          {!message.content && isStreaming && <span className="typing-cursor" />}
        </div>

        {/* 操作按钮 */}
        {!isStreaming && message.content && (
          <div className={clsx(
            'flex items-center gap-1.5 px-0.5',
            isUser ? 'flex-row-reverse' : 'flex-row'
          )}>
            <button onClick={handleCopy}
              className="p-1 rounded-md text-gray-500 hover:text-gray-300 transition-colors">
              {copied ? <Check size={13} className="text-green-400" /> : <Copy size={13} />}
            </button>

            {!isUser && onRegenerate && !isTemp && (
              <button onClick={(e) => { e.stopPropagation(); onRegenerate() }}
                className="p-1 rounded-md text-gray-500 hover:text-gray-300 transition-colors">
                <RefreshCw size={13} />
              </button>
            )}

            {!isTemp && (
              <button onClick={handleDelete}
                className="p-1 rounded-md text-gray-500 hover:text-red-400 transition-colors">
                <Trash2 size={13} />
              </button>
            )}

            <span className="text-[10px] text-gray-600 px-0.5">
              {new Date(message.created_at).toLocaleTimeString('zh-CN', {
                hour: '2-digit', minute: '2-digit'
              })}
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
