import React, { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowDown, ChevronLeft, MoreVertical, RefreshCw, Trash2 } from 'lucide-react'
import { useAuthStore, useChatStore, useCharacterStore, useUIStore, getToken } from '../store'
import MessageBubble from '../components/chat/MessageBubble'
import ChatInput from '../components/chat/ChatInput'
import Avatar from '../components/ui/Avatar'
import Modal from '../components/ui/Modal'
import { renderRolePlaceholders } from '../utils/placeholderRender'

const BOTTOM_THRESHOLD = 96
const REATTACH_THRESHOLD = 8

function getAuthHeaders() {
  try {
    const token = getToken()
    return token ? { Authorization: `Bearer ${token}` } : {}
  } catch {
    return {}
  }
}

export default function ChatPage() {
  const { chatId } = useParams()
  const navigate = useNavigate()
  const { showToast } = useUIStore()
  const user = useAuthStore(state => state.user)

  const {
    messages,
    loading,
    streaming,
    fetchMessages,
    sendMessage,
    sendStoryMessage,
    deleteChat,
    deleteMessageCascade,
    regenerate,
    fetchStoryStatus,
    retryStoryScheduler,
    storyStatus,
    schedulerRetrying,
  } = useChatStore()

  const [chat, setChat] = useState(null)
  const [character, setCharacter] = useState(null)
  const [showMenu, setShowMenu] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showJumpToBottom, setShowJumpToBottom] = useState(false)
  const messagesContainerRef = useRef(null)
  const stickToBottomRef = useRef(true)
  const touchStartYRef = useRef(null)
  const lastResumeSyncRef = useRef(0)

  const getDistanceToBottom = () => {
    const container = messagesContainerRef.current
    if (!container) return 0

    return Math.max(0, container.scrollHeight - container.scrollTop - container.clientHeight)
  }

  const updateBottomState = () => {
    const container = messagesContainerRef.current
    const canScroll = Boolean(container && container.scrollHeight > container.clientHeight + 1)
    const threshold = stickToBottomRef.current ? BOTTOM_THRESHOLD : REATTACH_THRESHOLD
    const pinned = !canScroll || getDistanceToBottom() <= threshold

    stickToBottomRef.current = pinned
    setShowJumpToBottom(canScroll && !pinned)
  }

  const pauseAutoScroll = () => {
    const container = messagesContainerRef.current
    if (!container || container.scrollHeight <= container.clientHeight + 1) return

    stickToBottomRef.current = false
    setShowJumpToBottom(true)
  }

  const handleWheel = event => {
    if (event.deltaY < 0) {
      pauseAutoScroll()
    }
  }

  const handleTouchStart = event => {
    touchStartYRef.current = event.touches[0]?.clientY ?? null
  }

  const handleTouchMove = event => {
    const touchStartY = touchStartYRef.current
    const touchY = event.touches[0]?.clientY

    if (touchStartY === null || touchY === undefined) return

    if (touchY - touchStartY > 8) {
      pauseAutoScroll()
    }
  }

  const scrollToBottom = (behavior = 'auto') => {
    const container = messagesContainerRef.current
    if (!container) return

    container.scrollTo({ top: container.scrollHeight, behavior })
  }

  const jumpToBottom = () => {
    stickToBottomRef.current = true
    setShowJumpToBottom(false)
    requestAnimationFrame(() => scrollToBottom('auto'))
  }

  useEffect(() => {
    stickToBottomRef.current = true
    setShowJumpToBottom(false)

    const loadChat = async () => {
      const headers = getAuthHeaders()
      const res = await fetch(`/api/chats/${chatId}`, { headers })
      if (!res.ok) {
        navigate('/chats')
        return
      }

      const data = await res.json()
      setChat(data)
      if (data.scheduler_enabled || data.schedulerEnabled) {
        fetchStoryStatus(chatId).catch(() => {})
      }

      const cachedCharacter = useCharacterStore
        .getState()
        .characters
        .find(item => item.id === data.character_id)

      if (cachedCharacter) {
        setCharacter(cachedCharacter)
        return
      }

      const characterRes = await fetch(`/api/characters/${data.character_id}`, { headers })
      if (characterRes.ok) {
        setCharacter(await characterRes.json())
      }
    }

    loadChat()
    fetchMessages(chatId)
  }, [chatId])

  useEffect(() => {
    if (!stickToBottomRef.current) return

    requestAnimationFrame(() => {
      scrollToBottom('auto')
    })
  }, [messages, streaming])

  useEffect(() => {
    const restoreBottomActions = () => {
      if (!stickToBottomRef.current) return

      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          scrollToBottom('auto')
        })
      })
    }

    const syncAfterResume = () => {
      restoreBottomActions()

      // focus, pageshow and visibilitychange often fire together. Coalesce
      // them into one quiet server reconciliation.
      const now = Date.now()
      if (now - lastResumeSyncRef.current < 300) return
      lastResumeSyncRef.current = now
      fetchMessages(chatId, { background: true }).catch(() => {})
    }

    const handleVisibilityChange = () => {
      if (!document.hidden) syncAfterResume()
    }

    window.addEventListener('focus', syncAfterResume)
    window.addEventListener('pageshow', syncAfterResume)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.visualViewport?.addEventListener('resize', restoreBottomActions)

    return () => {
      window.removeEventListener('focus', syncAfterResume)
      window.removeEventListener('pageshow', syncAfterResume)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      window.visualViewport?.removeEventListener('resize', restoreBottomActions)
    }
  }, [chatId, fetchMessages])

  const handleSend = async content => {
    stickToBottomRef.current = true
    setShowJumpToBottom(false)

    try {
      if (chat?.scheduler_enabled || chat?.schedulerEnabled) {
        await sendStoryMessage(chatId, content)
      } else {
        await sendMessage(chatId, content)
      }
    } catch (err) {
      showToast(err.message || '发送失败', 'error')
      throw err
    }
  }

  const isStoryChat = Boolean(chat?.scheduler_enabled || chat?.schedulerEnabled)
  const storyState = storyStatus?.State || storyStatus?.state
  const schedulerStatus = storyState?.SchedulerStatus || storyState?.scheduler_status
  const failureCount = storyState?.FailureCount ?? storyState?.failure_count ?? 0
  const latestFailure = storyStatus?.LatestFailure || storyStatus?.latestFailure
  const failureMessage = latestFailure?.ErrorMessage || latestFailure?.error_message
  const failureCode = latestFailure?.ErrorCode || latestFailure?.error_code

  const handleRetryScheduler = async () => {
    try {
      await retryStoryScheduler(chatId)
      showToast('调度重试完成', 'success')
    } catch (err) {
      showToast(err.message || '调度重试失败', 'error')
    }
  }

  const latestUserMessageId = !streaming
    ? [...messages].reverse().find(msg => msg.role === 'user')?.id || null
    : null

  const latestAssistantMessageId = !streaming
    ? [...messages].reverse().find(msg => msg.role === 'assistant')?.id || null
    : null

  const handleRetryLastRequest = async () => {
    if (!latestUserMessageId) {
      showToast('暂无可重试的上一条请求', 'error')
      return
    }

    try {
      await regenerate(chatId)
    } catch (err) {
      showToast(err.message || '重新发送失败', 'error')
    }
  }

  const handleRegenerate = async () => {
    try {
      await regenerate(chatId)
    } catch (err) {
      showToast(err.message || '重新生成失败', 'error')
    }
  }

  const handleDeleteChat = async () => {
    try {
      await deleteChat(chatId)
      navigate('/chats')
    } catch {
      showToast('删除失败', 'error')
    }
  }

  // Keep the synthetic opening message visible during the first round-trip.
  const hasPersistedMessages = messages.some(msg => !String(msg.id || '').startsWith('temp-'))
  const displayScenario = renderRolePlaceholders(character?.scenario, { character, user })
  const displayFirstMsg = renderRolePlaceholders(character?.first_msg, { character, user })
  const showOpeningScene = !loading && displayScenario && !hasPersistedMessages
  const showFirstMsg = !loading && displayFirstMsg && !hasPersistedMessages

  return (
    <div className="relative flex h-full min-h-0 flex-col bg-dark-400">
      <div className="glass border-b border-surface-border px-4 flex items-center gap-3 pt-[env(safe-area-inset-top)] h-[calc(56px+env(safe-area-inset-top))]">
        <button onClick={() => navigate('/chats')} className="btn-ghost p-2 -ml-2">
          <ChevronLeft size={22} />
        </button>

        {character && (
          <Avatar name={character.name} src={character.avatar_url} size="sm" />
        )}

        <div className="flex-1 min-w-0">
          <h2 className="font-semibold text-sm truncate">
            {character?.name || chat?.title || '...'}
          </h2>
          {streaming && (
            <span className="text-xs text-primary-400 flex items-center gap-1">
              <span className="w-1.5 h-1.5 bg-primary-400 rounded-full animate-pulse" />
              正在输入...
            </span>
          )}
        </div>

        <button onClick={() => setShowMenu(true)} className="btn-ghost p-2 -mr-2">
          <MoreVertical size={20} />
        </button>
      </div>

      {isStoryChat && storyState && schedulerStatus !== 'failed' && schedulerStatus !== 'paused' && schedulerStatus !== 'conflict' && (
        <div className="mx-4 mt-3 rounded-xl border border-primary-500/20 bg-primary-500/5 px-3 py-2 text-xs text-gray-300">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="font-medium text-primary-300">剧情状态：{schedulerStatus || 'ready'}</span>
            {(storyState.CurrentScene || storyState.current_scene) && <span>场景：{storyState.CurrentScene || storyState.current_scene}</span>}
            {(storyState.Route || storyState.route) && <span>路线：{storyState.Route || storyState.route}</span>}
            {(storyState.ActiveEvent || storyState.active_event) && <span>事件：{storyState.ActiveEvent || storyState.active_event}</span>}
          </div>
        </div>
      )}

      {isStoryChat && (schedulerStatus === 'failed' || schedulerStatus === 'paused' || schedulerStatus === 'conflict') && (
        <div className="mx-4 mt-3 rounded-xl border border-red-500/30 bg-red-500/10 px-3 py-2.5 text-sm text-red-200">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="font-medium">剧情调度失败</p>
              <p className="mt-0.5 text-xs text-red-300/80">失败次数：{failureCount}，上一份成功状态未被修改。</p>
              {(failureCode || failureMessage) && (
                <p className="mt-1 max-w-[36rem] truncate text-xs text-red-300/70" title={failureMessage}>
                  {failureCode ? `${failureCode}：` : ''}{failureMessage || '调度失败'}
                </p>
              )}
            </div>
            <button
              type="button"
              onClick={handleRetryScheduler}
              disabled={schedulerRetrying || streaming}
              className="rounded-lg border border-red-400/40 px-3 py-1.5 text-xs font-medium text-red-100 hover:bg-red-500/20 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {schedulerRetrying ? '重试中…' : '重试调度'}
            </button>
          </div>
        </div>
      )}

      <div
        ref={messagesContainerRef}
        onScroll={updateBottomState}
        onWheel={handleWheel}
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        className="min-h-0 flex-1 overflow-y-auto py-4 space-y-4"
      >
        {showOpeningScene && (
          <div className="px-4 message-enter">
            <div className="mx-auto max-w-2xl rounded-2xl border border-surface-border bg-surface/50 px-4 py-3 text-center">
              <p className="text-[11px] font-medium uppercase tracking-[0.24em] text-gray-500">
                场景设定
              </p>
              <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-gray-300">
                {displayScenario}
              </p>
            </div>
          </div>
        )}

        {showFirstMsg && (
          <div className="flex gap-2.5 px-4 message-enter">
            <Avatar name={character.name} src={character.avatar_url} size="sm" className="mt-0.5" />
            <div className="flex flex-col gap-1 max-w-[78%]">
              <span className="text-xs text-gray-500 px-1">{character.name}</span>
              <div className="bubble-ai chat-text leading-relaxed whitespace-pre-wrap">
                {displayFirstMsg}
              </div>
            </div>
          </div>
        )}

        {messages.map(msg => (
          <MessageBubble
            key={msg.id}
            message={msg}
            character={character}
            statusBarStyle={{ bg: chat?.status_bar_bg, fg: chat?.status_bar_fg }}
            onRegenerate={!isStoryChat && msg.id === latestAssistantMessageId ? handleRegenerate : undefined}
            onRetry={!isStoryChat && msg.id === latestUserMessageId ? handleRetryLastRequest : undefined}
            onDeleteCascade={!isStoryChat ? msgId => deleteMessageCascade(chatId, msgId) : undefined}
          />
        ))}

      </div>

      {showJumpToBottom && (
        <button
          type="button"
          onClick={jumpToBottom}
          aria-label="回到底部"
          title="回到底部"
          className="absolute bottom-[calc(env(safe-area-inset-bottom,0px)+7rem)] left-1/2 z-20 flex h-10 w-10 -translate-x-1/2 items-center justify-center rounded-full border border-surface-border bg-surface/95 text-gray-300 shadow-lg shadow-black/30 backdrop-blur transition-all hover:border-primary-500/50 hover:text-primary-300 active:scale-95"
        >
          <ArrowDown size={18} />
        </button>
      )}

      <ChatInput onSend={handleSend} disabled={loading || streaming} />

      <Modal open={showMenu} onClose={() => setShowMenu(false)} title="对话操作">
        <div className="space-y-2">
          <button
            onClick={() => {
              setShowMenu(false)
              fetchMessages(chatId)
            }}
            className="w-full flex items-center gap-3 p-3 rounded-xl hover:bg-surface-hover transition-colors text-left"
          >
            <RefreshCw size={18} className="text-gray-400" />
            <span>刷新消息</span>
          </button>
          <button
            onClick={() => {
              setShowMenu(false)
              setShowDeleteConfirm(true)
            }}
            className="w-full flex items-center gap-3 p-3 rounded-xl hover:bg-red-500/10 transition-colors text-left text-red-400"
          >
            <Trash2 size={18} />
            <span>删除对话</span>
          </button>
        </div>
      </Modal>

      <Modal open={showDeleteConfirm} onClose={() => setShowDeleteConfirm(false)} title="删除对话">
        <p className="text-gray-400 mb-6">
          确认删除这个对话吗？所有消息将被永久删除，无法恢复。
        </p>
        <div className="flex gap-3">
          <button
            onClick={() => setShowDeleteConfirm(false)}
            className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300 hover:bg-surface-hover transition-colors"
          >
            取消
          </button>
          <button
            onClick={handleDeleteChat}
            className="flex-1 py-3 rounded-xl bg-red-500 hover:bg-red-400 text-white font-medium transition-colors"
          >
            删除
          </button>
        </div>
      </Modal>
    </div>
  )
}
