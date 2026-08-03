import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { MessageSquare, Trash2, Plus, Search, Sparkles, Loader2 } from 'lucide-react'
import { useAuthStore, useChatStore, useCharacterStore, useUIStore, useSettingsStore } from '../store'
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
  const { settings, fetchSettings } = useSettingsStore()
  const [search, setSearch] = useState('')
  const [showNewChat, setShowNewChat] = useState(false)
  const [deletingId, setDeletingId] = useState(null)
  const [pendingStoryCharacter, setPendingStoryCharacter] = useState(null)
  const [initializingStory, setInitializingStory] = useState(false)

  useEffect(() => {
    fetchChats()
    fetchCharacters()
    fetchSettings().catch(() => {})
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

  const handleNewStoryChat = async (character) => {
    if (initializingStory) return
    setInitializingStory(true)
    const { createStoryChat } = useChatStore.getState()
    try {
      const compilerModel = settings?.story_compiler_model || settings?.default_model || 'gpt-4o-mini'
      const result = await createStoryChat({
        character_id: character.id,
        title: `与${character.name}的复杂剧情`,
        compiler_model: compilerModel,
        prompt_version: 'story-manifest-v1',
        compile_only_text: character.description || '',
      })
      const chat = result.chat || result.Chat
      setShowNewChat(false)
      navigate(`/chats/${chat?.id || chat?.ID || result.id || result.ID}`)
    } catch (err) {
      showToast(err.message || '复杂剧情初始化失败', 'error')
    } finally {
      setInitializingStory(false)
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

  const handleSelectCharacter = character => {
    if (character.tags?.includes('复杂剧情')) {
      setPendingStoryCharacter(character)
      return
    }
    handleNewChat(character)
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
              {/* 删除对话：复杂剧情也允许删除整个会话 */}
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
              <div
                key={char.id}
                onClick={() => handleSelectCharacter(char)}
                className="w-full flex items-center gap-3 p-3 rounded-xl
                           hover:bg-surface-hover transition-all duration-150 text-left cursor-pointer"
              >
                <Avatar name={char.name} src={char.avatar_url} size="md" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="font-medium text-sm">{char.name}</p>
                    {char.tags?.includes('复杂剧情') && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/15 px-2 py-0.5 text-[10px] text-amber-300">
                        <Sparkles size={10} />复杂剧情
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-gray-500 truncate">
                    {renderRolePlaceholders(char.description, { character: char, user })}
                  </p>
                </div>
                <span className="shrink-0 text-xs text-gray-500">开始</span>
              </div>
            ))}
          </div>
        )}
      </Modal>
      <Modal
        open={!!pendingStoryCharacter || initializingStory}
        onClose={() => {
          if (!initializingStory) setPendingStoryCharacter(null)
        }}
        title={initializingStory ? '初始化复杂剧情' : '进入动态模式？'}
      >
        {initializingStory ? (
          <div className="py-8 text-center space-y-4">
            <Loader2 size={38} className="mx-auto animate-spin text-primary-400" />
            <div>
              <p className="text-base font-medium text-gray-100">正在初始化剧情世界</p>
              <p className="mt-2 text-sm text-gray-500">正在调用初始化模型编译角色卡，请不要关闭页面…</p>
            </div>
          </div>
        ) : pendingStoryCharacter && (
          <div className="space-y-5">
            <div className="flex items-center gap-3">
              <Avatar name={pendingStoryCharacter.name} src={pendingStoryCharacter.avatar_url} size="lg" />
              <div>
                <p className="font-medium text-gray-100">{pendingStoryCharacter.name}</p>
                <p className="mt-1 text-xs text-amber-300">这是复杂剧情角色卡</p>
              </div>
            </div>
            <p className="text-sm leading-6 text-gray-300">
              是否进入动态模式？动态模式会调用初始化模型解析角色卡，并启用剧情状态、事件和调度模型。
            </p>
            <div className="flex gap-3">
              <button
                onClick={() => {
                  const character = pendingStoryCharacter
                  setPendingStoryCharacter(null)
                  handleNewChat(character)
                }}
                className="flex-1 rounded-xl border border-surface-border py-2.5 text-sm text-gray-300 hover:bg-surface-hover"
              >
                否，普通聊天
              </button>
              <button
                onClick={() => {
                  const character = pendingStoryCharacter
                  setPendingStoryCharacter(null)
                  handleNewStoryChat(character)
                }}
                className="flex-1 rounded-xl bg-primary-600 py-2.5 text-sm text-white hover:bg-primary-500"
              >
                是，进入动态模式
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
