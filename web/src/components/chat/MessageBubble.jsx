import React, { useState } from 'react'
import clsx from 'clsx'
import Avatar from '../ui/Avatar'
import MessageContent from './MessageContent'
import Modal from '../ui/Modal'
import { Trash2, Copy, Check, RefreshCw, ChevronDown, ChevronRight, Activity } from 'lucide-react'

function formatDurationSeconds(seconds) {
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds < 0) return null
  return `${seconds.toFixed(1)}s`
}

// 从 AI 回复中切出状态栏部分。以「【状态栏】」标题为锚点：
// 标题之前为正文，标题及之后为状态栏块。顺带剥掉残留的 ``` 或 ''' 代码块围栏行。
function splitStatusBar(content) {
  if (typeof content !== 'string') return { body: content, statusBar: null }
  const idx = content.lastIndexOf('【状态栏】')
  if (idx === -1) return { body: content, statusBar: null }
  // 向上回溯到该行行首，连同可能的围栏行一起划入状态栏块
  let start = content.lastIndexOf('\n', idx - 1)
  start = start === -1 ? 0 : start + 1
  // 若状态栏前一行是 ``` 或 ''' 围栏，把它也并进去（一并剥离）
  let bodyEnd = start
  const body = content.slice(0, bodyEnd)
  let statusBar = content.slice(bodyEnd)
  // 去掉围栏标记行（``` 或 '''）
  statusBar = statusBar.replace(/^[ \t]*(?:```|''')[^\n]*\n?/gm, '').trim()
  return { body: body.replace(/[ \t]*(?:```|''')[^\n]*\n?$/g, '').trimEnd(), statusBar }
}

export default function MessageBubble({ message, character, statusBarStyle, onRegenerate, onRetry, onDeleteCascade }) {
  const [copied, setCopied] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [statusOpen, setStatusOpen] = useState(false)

  const isUser = message.role === 'user'
  const isStreaming = message.isStreaming
  const isTemp = message.id?.startsWith('temp')
  const durationLabel = !isUser ? formatDurationSeconds(message.response_time_seconds) : null
  const legacyParts = !isUser ? splitStatusBar(message.content) : { body: message.content, statusBar: null }
  const bodyContent = message.status_bar ? message.content : legacyParts.body
  const statusBarContent = message.status_bar || legacyParts.statusBar
  const hasDisplayContent = Boolean(message.content || statusBarContent)

  const handleCopy = async (e) => {
    e.stopPropagation()
    const copyContent = isUser
      ? message.content
      : [bodyContent, statusBarContent].filter(Boolean).join('\n\n')
    await navigator.clipboard.writeText(copyContent)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleDelete = (e) => {
    e.stopPropagation()
    if (isTemp) return
    setShowDeleteConfirm(true)
  }

  const confirmDelete = () => {
    setShowDeleteConfirm(false)
    if (onDeleteCascade) onDeleteCascade(message.id)
  }

  return (
    <>
      <div
        className={clsx(
          'flex gap-2.5 px-4 message-enter',
          isUser ? 'flex-row-reverse' : 'flex-row'
        )}
      >
        {!isUser && (
          <Avatar name={character?.name} src={character?.avatar_url}
            size="sm" className="mt-0.5 flex-shrink-0" />
        )}

        <div className={clsx('max-w-[78%] flex flex-col gap-1', isUser ? 'items-end' : 'items-start')}>
          {!isUser && (
            <span className="text-xs text-gray-500 px-1">{character?.name || 'AI'}</span>
          )}

          <div className={clsx(
            'chat-text leading-relaxed break-words',
            isUser ? 'bubble-user' : 'bubble-ai',
            isStreaming && !message.content && 'min-w-[60px] min-h-[36px]'
          )}>
            {isUser ? (
              <span className="whitespace-pre-wrap">{message.content}</span>
            ) : (
              <>
                {(() => {
                  // 流式输出过程中不拆分，避免状态栏闪烁；输出完成后再折叠
                  if (isStreaming) {
                    return <MessageContent content={message.content} isUser={false} />
                  }
                  const sbBg = statusBarStyle?.bg || ''
                  const sbFg = statusBarStyle?.fg || ''
                  const sbBlockStyle = sbBg ? { backgroundColor: sbBg, borderColor: sbBg } : undefined
                  const sbTextStyle = sbFg ? { color: sbFg } : undefined
                  return (
                    <>
                      <MessageContent content={bodyContent} isUser={false} />
                      {statusBarContent && (
                        <div
                          className="status-bar-panel mt-2 overflow-hidden rounded-lg border"
                          style={sbBlockStyle}>
                          <button
                            onClick={(e) => { e.stopPropagation(); setStatusOpen(o => !o) }}
                            style={sbTextStyle}
                            className="status-bar-text status-bar-toggle flex w-full select-none items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium transition-colors">
                            <Activity size={12} />
                            <span>状态栏</span>
                            <span className="ml-auto">
                              {statusOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                            </span>
                          </button>
                          {statusOpen && (
                            <pre
                              style={sbTextStyle}
                              className="status-bar-text status-bar-divider whitespace-pre-wrap break-words border-t px-3 py-2 font-mono text-[12px] leading-relaxed">
                              {statusBarContent}
                            </pre>
                          )}
                        </div>
                      )}
                    </>
                  )
                })()}
                {isStreaming && <span className="typing-cursor" />}
              </>
            )}
            {!message.content && isStreaming && <span className="typing-cursor" />}
          </div>

          {!isStreaming && hasDisplayContent && (
            <div className={clsx(
              'flex items-center gap-1.5 px-0.5',
              isUser ? 'flex-row-reverse' : 'flex-row'
            )}>
              {durationLabel && (
                <span className="text-[10px] text-gray-500 px-0.5 select-none" title="AI 响应耗时">
                  {durationLabel}
                </span>
              )}

              <button onClick={handleCopy}
                className="p-1 rounded-md text-gray-500 hover:text-gray-300 transition-colors">
                {copied ? <Check size={13} className="text-green-400" /> : <Copy size={13} />}
              </button>

              {!isUser && onRegenerate && !isTemp && (
                <button onClick={(e) => { e.stopPropagation(); onRegenerate() }}
                  title="重新生成"
                  className="p-1 rounded-md text-gray-500 hover:text-gray-300 transition-colors">
                  <RefreshCw size={13} />
                </button>
              )}

              {isUser && onRetry && (
                <button onClick={(e) => { e.stopPropagation(); onRetry() }}
                  title="重新生成"
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

      {/* 删除确认弹窗 */}
      <Modal open={showDeleteConfirm} onClose={() => setShowDeleteConfirm(false)} title="删除消息">
        <p className="text-sm text-gray-400 mb-5">
          将删除此消息及之后的所有消息，此操作无法撤销。
        </p>
        <div className="flex gap-3">
          <button onClick={() => setShowDeleteConfirm(false)}
            className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                       hover:bg-surface-hover transition-colors">
            取消
          </button>
          <button onClick={confirmDelete}
            className="flex-1 py-3 rounded-xl bg-red-500 hover:bg-red-400
                       text-white font-medium transition-colors">
            删除
          </button>
        </div>
      </Modal>
    </>
  )
}
