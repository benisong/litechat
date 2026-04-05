import React, { useEffect, useState } from 'react'
import { BookOpen, Plus, Trash2, ChevronRight, ChevronLeft, ToggleLeft, ToggleRight } from 'lucide-react'
import { useWorldBookStore, useUIStore } from '../store'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'

export default function WorldBooksPage() {
  const { worldBooks, currentBook, fetchWorldBooks, createWorldBook, deleteWorldBook,
          fetchWorldBook, createEntry, updateEntry, deleteEntry } = useWorldBookStore()
  const { showToast } = useUIStore()

  const [view, setView] = useState('list') // 'list' | 'book'
  const [showNewBook, setShowNewBook] = useState(false)
  const [newBookForm, setNewBookForm] = useState({ name: '', description: '' })
  const [showNewEntry, setShowNewEntry] = useState(false)
  const [entryForm, setEntryForm] = useState({ keys: '', content: '', enabled: true, priority: 0 })
  const [editEntry, setEditEntry] = useState(null)

  useEffect(() => { fetchWorldBooks() }, [])

  const handleOpenBook = async (id) => {
    await fetchWorldBook(id)
    setView('book')
  }

  const handleCreateBook = async () => {
    if (!newBookForm.name.trim()) { showToast('请填写世界书名称', 'error'); return }
    try {
      await createWorldBook(newBookForm)
      setShowNewBook(false)
      setNewBookForm({ name: '', description: '' })
      showToast('世界书创建成功', 'success')
    } catch { showToast('创建失败', 'error') }
  }

  const handleDeleteBook = async (id, e) => {
    e.stopPropagation()
    try {
      await deleteWorldBook(id)
      showToast('世界书已删除', 'success')
    } catch { showToast('删除失败', 'error') }
  }

  const handleSaveEntry = async () => {
    if (!entryForm.content.trim()) { showToast('请填写条目内容', 'error'); return }
    try {
      if (editEntry) {
        await updateEntry(editEntry.id, { ...entryForm, world_book_id: currentBook.id })
        showToast('保存成功', 'success')
      } else {
        await createEntry(currentBook.id, entryForm)
        showToast('条目已添加', 'success')
      }
      setShowNewEntry(false)
      setEditEntry(null)
      setEntryForm({ keys: '', content: '', enabled: true, priority: 0 })
    } catch { showToast('保存失败', 'error') }
  }

  const handleToggleEntry = async (entry) => {
    try {
      await updateEntry(entry.id, { ...entry, enabled: !entry.enabled })
    } catch { showToast('操作失败', 'error') }
  }

  if (view === 'book' && currentBook) {
    return (
      <div className="flex flex-col h-full">
        <div className="px-4 pt-12 pb-4 flex items-center gap-3">
          <button onClick={() => setView('list')} className="btn-ghost p-2 -ml-2">
            <ChevronLeft size={22} />
          </button>
          <div className="flex-1">
            <h1 className="text-xl font-bold">{currentBook.name}</h1>
            {currentBook.description && (
              <p className="text-xs text-gray-500">{currentBook.description}</p>
            )}
          </div>
          <button
            onClick={() => { setEntryForm({ keys: '', content: '', enabled: true, priority: 0 }); setEditEntry(null); setShowNewEntry(true) }}
            className="btn-primary flex items-center gap-1.5 py-2 px-3 text-sm"
          >
            <Plus size={15} /> 添加条目
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 space-y-2">
          {(!currentBook.entries || currentBook.entries.length === 0) ? (
            <EmptyState icon={BookOpen} title="还没有条目" description="添加关键词触发的知识条目" />
          ) : currentBook.entries.map(entry => (
            <div
              key={entry.id}
              className="card p-4 cursor-pointer hover:bg-surface-hover transition-colors"
              onClick={() => { setEntryForm({ ...entry }); setEditEntry(entry); setShowNewEntry(true) }}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex-1 min-w-0">
                  {entry.keys && (
                    <div className="flex gap-1.5 flex-wrap mb-2">
                      {entry.keys.split(',').map(k => k.trim()).filter(Boolean).map(k => (
                        <span key={k} className="text-[11px] bg-primary-500/20 text-primary-300
                                                  px-2 py-0.5 rounded-full border border-primary-500/20">
                          {k}
                        </span>
                      ))}
                    </div>
                  )}
                  <p className="text-sm text-gray-300 line-clamp-3">{entry.content}</p>
                  <p className="text-xs text-gray-600 mt-1">优先级 {entry.priority}</p>
                </div>
                <div className="flex items-center gap-2 flex-shrink-0">
                  <button onClick={e => { e.stopPropagation(); handleToggleEntry(entry) }}>
                    {entry.enabled
                      ? <ToggleRight size={20} className="text-primary-400" />
                      : <ToggleLeft size={20} className="text-gray-600" />
                    }
                  </button>
                  <button onClick={e => { e.stopPropagation(); deleteEntry(entry.id) }}
                    className="p-1.5 text-gray-600 hover:text-red-400 transition-colors">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* 条目编辑弹窗 */}
        <Modal open={showNewEntry} onClose={() => { setShowNewEntry(false); setEditEntry(null) }}
          title={editEntry ? '编辑条目' : '新建条目'}>
          <div className="space-y-4">
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">触发关键词（逗号分隔）</label>
              <input className="w-full input-base" value={entryForm.keys}
                onChange={e => setEntryForm(f => ({ ...f, keys: e.target.value }))}
                placeholder="例如：魔法，法师，魔法学院" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">注入内容 *</label>
              <textarea className="w-full input-base resize-none" rows={5}
                value={entryForm.content}
                onChange={e => setEntryForm(f => ({ ...f, content: e.target.value }))}
                placeholder="当对话中出现关键词时，此内容将被注入到上下文中" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-gray-400 mb-1.5">优先级</label>
                <input type="number" className="w-full input-base" value={entryForm.priority}
                  onChange={e => setEntryForm(f => ({ ...f, priority: parseInt(e.target.value) || 0 }))} />
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1.5">状态</label>
                <label className="flex items-center gap-2 cursor-pointer h-11 px-3 bg-surface
                                   border border-surface-border rounded-xl">
                  <input type="checkbox" checked={entryForm.enabled}
                    onChange={e => setEntryForm(f => ({ ...f, enabled: e.target.checked }))}
                    className="w-4 h-4 rounded accent-violet-500" />
                  <span className="text-sm">启用</span>
                </label>
              </div>
            </div>
            <div className="flex gap-3 pt-2">
              <button onClick={() => { setShowNewEntry(false); setEditEntry(null) }}
                className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                           hover:bg-surface-hover transition-colors">取消</button>
              <button onClick={handleSaveEntry} className="flex-1 btn-primary py-3">保存</button>
            </div>
          </div>
        </Modal>
      </div>
    )
  }

  // 世界书列表视图
  return (
    <div className="flex flex-col h-full">
      <div className="px-4 pt-12 pb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold">世界书</h1>
        <button onClick={() => setShowNewBook(true)} className="btn-primary flex items-center gap-2 py-2 px-4 text-sm">
          <Plus size={16} /> 新建
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-4 space-y-2">
        {worldBooks.length === 0 ? (
          <EmptyState icon={BookOpen} title="还没有世界书" description="创建知识库，为角色扮演注入背景知识" />
        ) : worldBooks.map(wb => (
          <div key={wb.id}
            onClick={() => handleOpenBook(wb.id)}
            className="card p-4 flex items-center gap-3 cursor-pointer
                       hover:bg-surface-hover active:scale-[0.99] transition-all duration-150">
            <div className="w-11 h-11 rounded-2xl bg-gradient-to-br from-amber-500/20 to-orange-500/20
                            border border-amber-500/20 flex items-center justify-center flex-shrink-0">
              <BookOpen size={20} className="text-amber-400" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-semibold text-sm">{wb.name}</p>
              <p className="text-xs text-gray-500 truncate">{wb.description || '暂无描述'}</p>
            </div>
            <div className="flex items-center gap-2">
              <button onClick={e => handleDeleteBook(wb.id, e)}
                className="p-2 text-gray-600 hover:text-red-400 transition-colors rounded-lg">
                <Trash2 size={15} />
              </button>
              <ChevronRight size={18} className="text-gray-600" />
            </div>
          </div>
        ))}
      </div>

      <Modal open={showNewBook} onClose={() => setShowNewBook(false)} title="新建世界书">
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">名称 *</label>
            <input className="w-full input-base" value={newBookForm.name}
              onChange={e => setNewBookForm(f => ({ ...f, name: e.target.value }))}
              placeholder="例如：奇幻世界背景" />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">描述</label>
            <textarea className="w-full input-base resize-none" rows={3}
              value={newBookForm.description}
              onChange={e => setNewBookForm(f => ({ ...f, description: e.target.value }))}
              placeholder="简短描述这个世界书的内容" />
          </div>
          <div className="flex gap-3 pt-2">
            <button onClick={() => setShowNewBook(false)}
              className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                         hover:bg-surface-hover transition-colors">取消</button>
            <button onClick={handleCreateBook} className="flex-1 btn-primary py-3">创建</button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
