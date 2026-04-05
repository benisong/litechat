import React, { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ChevronLeft, MoreVertical, Trash2, RefreshCw } from 'lucide-react'
import { useChatStore, useCharacterStore, useUIStore } from '../store'
import MessageBubble from '../components/chat/MessageBubble'
import ChatInput from '../components/chat/ChatInput'
import Avatar from '../components/ui/Avatar'
import Modal from '../components/ui/Modal'

export default function ChatPage() {
  const { chatId } = useParams()
  const navigate = useNavigate()
  const { showToast } = useUIStore()

  const { currentChat, messages, streaming, fetchMessages, sendMessage, deleteChat } = useChatStore()
  const { characters } = useCharacterStore()

  const [chat, setChat] = useState(null)
  const [character, setCharacter] = useState(null)
  const [showMenu, setShowMenu] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const messagesEndRef = useRef(null)

  useEffect(() => {
    // 加载对话信息
    const loadChat = async () => {
      const res = await fetch(`/api/chats/${chatId}`)
      if (!res.ok) { navigate('/chats'); return }
      const data = await res.json()
      setChat(data)

      // 查找角色
      const chars = useCharacterStore.getState().characters
      const char = chars.find(c => c.id === data.character_id)
      if (char) {
        setCharacter(char)
      } else {
        // 从 API 获取
        const r = await fetch(`/api/characters/${data.character_id}`)
        if (r.ok) setCharacter(await r.json())
      }
    }

    loadChat()
    fetchMessages(chatId)
  }, [chatId])

  // 自动滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSend = async (content) => {
    try {
      await sendMessage(chatId, content)
    } catch (err) {
      showToast(err.message || '发送失败', 'error')
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

  // 处理角色开场白（消息为空时展示）
  const showFirstMsg = messages.length === 0 && character?.first_msg && !streaming

  return (
    <div className="flex flex-col h-dvh bg-dark-400">
      {/* 顶部导航 */}
      <div className="glass border-b border-surface-border px-4 flex items-center gap-3
                      pt-[env(safe-area-inset-top)] h-[calc(56px+env(safe-area-inset-top))]">
        <button
          onClick={() => navigate('/chats')}
          className="btn-ghost p-2 -ml-2"
        >
          <ChevronLeft size={22} />
        </button>

        {character && (
          <Avatar name={character.name} src={character.avatar_url} size="sm" />
        )}

        <div className="flex-1 min-w-0">
          <h2 className="font-semibold text-sm truncate">
            {character?.name || chat?.title || '…'}
          </h2>
          {streaming && (
            <span className="text-xs text-primary-400 flex items-center gap-1">
              <span className="w-1.5 h-1.5 bg-primary-400 rounded-full animate-pulse" />
              正在输入…
            </span>
          )}
        </div>

        <button
          onClick={() => setShowMenu(true)}
          className="btn-ghost p-2 -mr-2"
        >
          <MoreVertical size={20} />
        </button>
      </div>

      {/* 消息列表 */}
      <div className="flex-1 overflow-y-auto py-4 space-y-4">
        {/* 角色开场白 */}
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

        {/* 消息列表 */}
        {messages.map(msg => (
          <MessageBubble key={msg.id} message={msg} character={character} />
        ))}

        <div ref={messagesEndRef} />
      </div>

      {/* 输入框 */}
      <ChatInput onSend={handleSend} disabled={streaming} />

      {/* 菜单弹窗 */}
      <Modal open={showMenu} onClose={() => setShowMenu(false)} title="对话操作">
        <div className="space-y-2">
          <button
            onClick={() => { setShowMenu(false); fetchMessages(chatId) }}
            className="w-full flex items-center gap-3 p-3 rounded-xl hover:bg-surface-hover
                       transition-colors text-left"
          >
            <RefreshCw size={18} className="text-gray-400" />
            <span>刷新消息</span>
          </button>
          <button
            onClick={() => { setShowMenu(false); setShowDeleteConfirm(true) }}
            className="w-full flex items-center gap-3 p-3 rounded-xl hover:bg-red-500/10
                       transition-colors text-left text-red-400"
          >
            <Trash2 size={18} />
            <span>删除对话</span>
          </button>
        </div>
      </Modal>

      {/* 删除确认弹窗 */}
      <Modal open={showDeleteConfirm} onClose={() => setShowDeleteConfirm(false)} title="删除对话">
        <p className="text-gray-400 mb-6">确认删除这个对话吗？所有消息将被永久删除，无法恢复。</p>
        <div className="flex gap-3">
          <button
            onClick={() => setShowDeleteConfirm(false)}
            className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                       hover:bg-surface-hover transition-colors"
          >
            取消
          </button>
          <button
            onClick={handleDeleteChat}
            className="flex-1 py-3 rounded-xl bg-red-500 hover:bg-red-400
                       text-white font-medium transition-colors"
          >
            删除
          </button>
        </div>
      </Modal>
    </div>
  )
}
