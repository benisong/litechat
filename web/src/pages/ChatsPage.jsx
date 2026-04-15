import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { MessageSquare, Trash2, Plus, Search } from 'lucide-react'
import { useAuthStore, useChatStore, useCharacterStore, useUIStore } from '../store'
import Avatar from '../components/ui/Avatar'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'
import clsx from 'clsx'
import { renderRolePlaceholders } from '../utils/placeholderRender'

export default function ChatsPage() {
  const navigate = useNavigate()
  const user = useAuthStore(state => state.user)
  const { chats, fetchChats, deleteChat } = useChatStore()
  const { characters, fetchCharacters } = useCharacterStore()
  const { showToast } = useUIStore()
  const [search, setSearch] = useState('')
  const [showNewChat, setShowNewChat] = useState(false)
  const [deletingId, setDeletingId] = useState(null)

  useEffect(() => {
    fetchChats()
    fetchCharacters()
  }, [])

  const filtered = chats.filter(c =>
    (c.title || '').toLowerCase().includes(search.toLowerCase()) ||
    (c.character?.name || '').toLowerCase().includes(search.toLowerCase())
  )

  const handleDelete = async (id, e) => {
    e.stopPropagation()
    setDeletingId(id)
    try {
      await deleteChat(id)
      showToast('对话已删除', 'success')
    } catch {
      showToast('删除失败', 'error')
    } finally {
      setDeletingId(null)
    }
  }

  const handleNewChat = async (character) => {
    const { createChat } = useChatStore.getState()
    try {
      const chat = await createChat(character.id, `与${character.name}的对话`)
      setShowNewChat(false)
      navigate(`/chats/${chat.id}`)
    } catch {
      showToast('创建对话失败', 'error')
    }
  }

  return (
    <div className="flex flex-col h-full">
      {/* 顶部标题 */}
      <div className="px-4 pt-12 pb-4">
        <h1 className="text-2xl font-bold mb-4">聊天</h1>
        {/* 搜索框 */}
        <div className="relative">
          <Search size={16} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-500" />
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="搜索对话…"
            className="w-full input-base pl-10 py-2.5 text-sm"
          />
        </div>
      </div>

      {/* 对话列表 */}
      <div className="flex-1 overflow-y-auto px-4 space-y-2">
        {filtered.length === 0 ? (
          <EmptyState
            icon={MessageSquare}
            title="还没有对话"
            description="选择一个角色开始聊天吧"
            action={
              <button
                onClick={() => setShowNewChat(true)}
                className="btn-primary"
              >
                开始新对话
              </button>
            }
          />
        ) : (
          filtered.map(chat => (
            <div
              key={chat.id}
              onClick={() => navigate(`/chats/${chat.id}`)}
              className="card flex items-center gap-3 p-3.5 cursor-pointer
                         hover:bg-surface-hover active:scale-[0.99]
                         transition-all duration-150"
            >
              <Avatar
                name={chat.character?.name}
                src={chat.character?.avatar_url}
                size="md"
              />
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between mb-0.5">
                  <span className="font-medium text-sm truncate">{chat.title}</span>
                  <span className="text-[10px] text-gray-500 flex-shrink-0 ml-2">
                    {new Date(chat.updated_at).toLocaleDateString('zh-CN', {
                      month: 'short', day: 'numeric'
                    })}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <p className="text-xs text-gray-500 truncate">
                    {chat.character?.name && (
                      <span className="text-primary-400 mr-1">{chat.character.name}</span>
                    )}
                    {chat.last_message || '暂无消息'}
                  </p>
                  {chat.msg_count > 0 && (
                    <span className="text-[10px] text-gray-600 flex-shrink-0 ml-2">
                      {chat.msg_count}条
                    </span>
                  )}
                </div>
              </div>
              {/* 删除按钮 */}
              <button
                onClick={e => handleDelete(chat.id, e)}
                className="p-2 text-gray-600 hover:text-red-400 transition-colors rounded-lg"
                disabled={deletingId === chat.id}
              >
                <Trash2 size={15} />
              </button>
            </div>
          ))
        )}
      </div>

      {/* 新建对话浮动按钮 */}
      {chats.length > 0 && (
        <button
          onClick={() => setShowNewChat(true)}
          className="fixed bottom-20 right-4 w-14 h-14 bg-primary-600 hover:bg-primary-500
                     rounded-2xl flex items-center justify-center shadow-xl shadow-primary-600/30
                     active:scale-90 transition-all duration-150 z-40"
        >
          <Plus size={24} />
        </button>
      )}

      {/* 选择角色弹窗 */}
      <Modal
        open={showNewChat}
        onClose={() => setShowNewChat(false)}
        title="选择角色开始聊天"
      >
        {characters.length === 0 ? (
          <div className="py-8 text-center">
            <p className="text-gray-500 mb-4">还没有角色卡，先去创建一个吧</p>
            <button
              onClick={() => { setShowNewChat(false); navigate('/characters/new') }}
              className="btn-primary"
            >
              创建角色
            </button>
          </div>
        ) : (
          <div className="space-y-2">
            {characters.map(char => (
              <button
                key={char.id}
                onClick={() => handleNewChat(char)}
                className="w-full flex items-center gap-3 p-3 rounded-xl
                           hover:bg-surface-hover active:scale-[0.99]
                           transition-all duration-150 text-left"
              >
                <Avatar name={char.name} src={char.avatar_url} size="md" />
                <div className="flex-1 min-w-0">
                  <p className="font-medium text-sm">{char.name}</p>
                  <p className="text-xs text-gray-500 truncate">
                    {renderRolePlaceholders(char.description, { character: char, user })}
                  </p>
                </div>
              </button>
            ))}
          </div>
        )}
      </Modal>
    </div>
  )
}
