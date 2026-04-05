import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Users, Plus, MessageSquare, Edit2, Trash2 } from 'lucide-react'
import { useCharacterStore, useChatStore, useUIStore } from '../store'
import Avatar from '../components/ui/Avatar'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'

export default function CharactersPage() {
  const navigate = useNavigate()
  const { characters, fetchCharacters, deleteCharacter } = useCharacterStore()
  const { createChat } = useChatStore()
  const { showToast } = useUIStore()
  const [deletingId, setDeletingId] = useState(null)
  const [selectedChar, setSelectedChar] = useState(null)

  useEffect(() => { fetchCharacters() }, [])

  const handleChat = async (char, e) => {
    e.stopPropagation()
    try {
      const chat = await createChat(char.id, `与${char.name}的对话`)
      navigate(`/chats/${chat.id}`)
    } catch {
      showToast('创建对话失败', 'error')
    }
  }

  const handleDelete = async () => {
    try {
      await deleteCharacter(selectedChar.id)
      showToast('角色已删除', 'success')
      setSelectedChar(null)
    } catch {
      showToast('删除失败', 'error')
    }
  }

  return (
    <div className="flex flex-col h-full">
      {/* 标题栏 */}
      <div className="px-4 pt-12 pb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold">角色</h1>
        <button
          onClick={() => navigate('/characters/new')}
          className="btn-primary flex items-center gap-2 py-2 px-4 text-sm"
        >
          <Plus size={16} />
          新建
        </button>
      </div>

      {/* 角色网格 */}
      <div className="flex-1 overflow-y-auto px-4">
        {characters.length === 0 ? (
          <EmptyState
            icon={Users}
            title="还没有角色卡"
            description="创建你的第一个 AI 角色"
            action={
              <button
                onClick={() => navigate('/characters/new')}
                className="btn-primary"
              >
                创建角色
              </button>
            }
          />
        ) : (
          <div className="grid grid-cols-2 gap-3 pb-4">
            {characters.map(char => (
              <div
                key={char.id}
                className="card p-4 flex flex-col gap-3 cursor-pointer
                           hover:bg-surface-hover active:scale-[0.98]
                           transition-all duration-150"
                onClick={() => setSelectedChar(char)}
              >
                {/* 头像 */}
                <div className="flex items-start justify-between">
                  <Avatar name={char.name} src={char.avatar_url} size="lg" />
                  {char.tags && (
                    <span className="text-[10px] bg-primary-500/20 text-primary-300
                                     px-2 py-0.5 rounded-full border border-primary-500/20">
                      {char.tags.split(',')[0]}
                    </span>
                  )}
                </div>

                {/* 名字和描述 */}
                <div>
                  <h3 className="font-semibold text-sm mb-1 truncate">{char.name}</h3>
                  <p className="text-xs text-gray-500 line-clamp-2">{char.description || '暂无描述'}</p>
                </div>

                {/* 操作按钮 */}
                <div className="flex gap-2 mt-auto">
                  <button
                    onClick={e => handleChat(char, e)}
                    className="flex-1 flex items-center justify-center gap-1.5 py-2 rounded-xl
                               bg-primary-600/20 text-primary-400 text-xs font-medium
                               hover:bg-primary-600/30 transition-colors"
                  >
                    <MessageSquare size={13} />
                    聊天
                  </button>
                  <button
                    onClick={e => { e.stopPropagation(); navigate(`/characters/${char.id}/edit`) }}
                    className="p-2 rounded-xl bg-surface-hover text-gray-400
                               hover:text-white transition-colors"
                  >
                    <Edit2 size={14} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 角色详情弹窗 */}
      <Modal
        open={!!selectedChar}
        onClose={() => setSelectedChar(null)}
        title={selectedChar?.name}
      >
        {selectedChar && (
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              <Avatar name={selectedChar.name} src={selectedChar.avatar_url} size="xl" />
              <div>
                <h3 className="text-xl font-bold">{selectedChar.name}</h3>
                {selectedChar.tags && (
                  <div className="flex gap-1 mt-1 flex-wrap">
                    {selectedChar.tags.split(',').map(tag => (
                      <span key={tag} className="text-xs bg-surface px-2 py-0.5 rounded-full text-gray-400 border border-surface-border">
                        {tag.trim()}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {selectedChar.description && (
              <div>
                <p className="text-xs text-gray-500 mb-1">描述</p>
                <p className="text-sm text-gray-300">{selectedChar.description}</p>
              </div>
            )}

            {selectedChar.personality && (
              <div>
                <p className="text-xs text-gray-500 mb-1">性格</p>
                <p className="text-sm text-gray-300">{selectedChar.personality}</p>
              </div>
            )}

            {selectedChar.first_msg && (
              <div>
                <p className="text-xs text-gray-500 mb-1">开场白</p>
                <p className="text-sm text-gray-300 italic">"{selectedChar.first_msg}"</p>
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <button
                onClick={e => { setSelectedChar(null); handleChat(selectedChar, e) }}
                className="flex-1 btn-primary flex items-center justify-center gap-2"
              >
                <MessageSquare size={16} />
                开始聊天
              </button>
              <button
                onClick={() => { setSelectedChar(null); navigate(`/characters/${selectedChar.id}/edit`) }}
                className="px-4 py-2.5 rounded-xl border border-surface-border text-gray-300
                           hover:bg-surface-hover transition-colors"
              >
                <Edit2 size={16} />
              </button>
              <button
                onClick={handleDelete}
                className="px-4 py-2.5 rounded-xl border border-red-500/30 text-red-400
                           hover:bg-red-500/10 transition-colors"
              >
                <Trash2 size={16} />
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
