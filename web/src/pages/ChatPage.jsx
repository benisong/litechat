import React, { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ChevronLeft, MoreVertical, RefreshCw, Trash2 } from 'lucide-react'
import { useChatStore, useCharacterStore, useUIStore } from '../store'
import MessageBubble from '../components/chat/MessageBubble'
import ChatInput from '../components/chat/ChatInput'
import Avatar from '../components/ui/Avatar'
import Modal from '../components/ui/Modal'

function getAuthHeaders() {
  try {
    const stored = localStorage.getItem('litechat-auth')
    const token = stored ? JSON.parse(stored)?.state?.token : null
    return token ? { Authorization: `Bearer ${token}` } : {}
  } catch {
    return {}
  }
}

export default function ChatPage() {
  const { chatId } = useParams()
  const navigate = useNavigate()
  const { showToast } = useUIStore()

  const {
    messages,
    loading,
    streaming,
    fetchMessages,
    sendMessage,
    deleteChat,
    deleteMessageCascade,
    regenerate,
  } = useChatStore()

  const [chat, setChat] = useState(null)
  const [character, setCharacter] = useState(null)
  const [showMenu, setShowMenu] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const messagesEndRef = useRef(null)

  const scrollToBottom = (behavior = 'auto') => {
    messagesEndRef.current?.scrollIntoView({ behavior, block: 'end' })
  }

  useEffect(() => {
    const loadChat = async () => {
      const headers = getAuthHeaders()
      const res = await fetch(`/api/chats/${chatId}`, { headers })
      if (!res.ok) {
        navigate('/chats')
        return
      }

      const data = await res.json()
      setChat(data)

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
    scrollToBottom('smooth')
  }, [messages])

  useEffect(() => {
    const restoreBottomActions = () => {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          scrollToBottom('auto')
        })
      })
    }

    const handleVisibilityChange = () => {
      if (!document.hidden) restoreBottomActions()
    }

    window.addEventListener('focus', restoreBottomActions)
    window.addEventListener('pageshow', restoreBottomActions)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.visualViewport?.addEventListener('resize', restoreBottomActions)

    return () => {
      window.removeEventListener('focus', restoreBottomActions)
      window.removeEventListener('pageshow', restoreBottomActions)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      window.visualViewport?.removeEventListener('resize', restoreBottomActions)
    }
  }, [])

  const handleSend = async content => {
    try {
      await sendMessage(chatId, content)
    } catch (err) {
      showToast(err.message || '发送失败', 'error')
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
  const showOpeningScene = !loading && character?.scenario && !hasPersistedMessages
  const showFirstMsg = !loading && character?.first_msg && !hasPersistedMessages

  return (
    <div className="flex h-full min-h-0 flex-col bg-dark-400">
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

      <div className="min-h-0 flex-1 overflow-y-auto py-4 space-y-4">
        {showOpeningScene && (
          <div className="px-4 message-enter">
            <div className="mx-auto max-w-2xl rounded-2xl border border-surface-border bg-surface/50 px-4 py-3 text-center">
              <p className="text-[11px] font-medium uppercase tracking-[0.24em] text-gray-500">
                场景设定
              </p>
              <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-gray-300">
                {character.scenario}
              </p>
            </div>
          </div>
        )}

        {showFirstMsg && (
          <div className="flex gap-2.5 px-4 message-enter">
            <Avatar name={character.name} src={character.avatar_url} size="sm" className="mt-0.5" />
            <div className="flex flex-col gap-1 max-w-[78%]">
              <span className="text-xs text-gray-500 px-1">{character.name}</span>
              <div className="bubble-ai text-sm leading-relaxed whitespace-pre-wrap">
                {character.first_msg}
              </div>
            </div>
          </div>
        )}

        {messages.map(msg => (
          <MessageBubble
            key={msg.id}
            message={msg}
            character={character}
            onRegenerate={msg.id === latestAssistantMessageId ? handleRegenerate : undefined}
            onRetry={msg.id === latestUserMessageId ? handleRetryLastRequest : undefined}
            onDeleteCascade={msgId => deleteMessageCascade(chatId, msgId)}
          />
        ))}

        <div ref={messagesEndRef} />
      </div>

      <ChatInput onSend={handleSend} disabled={streaming} />

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
