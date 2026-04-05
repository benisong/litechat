import React, { useEffect, useState } from 'react'
import { BookOpen, Plus, Trash2, ChevronRight, ChevronLeft, ToggleLeft, ToggleRight,
         ChevronDown, ChevronUp, Pin, Search as SearchIcon, Globe, User } from 'lucide-react'
import { useWorldBookStore, useCharacterStore, useUIStore } from '../store'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'
import clsx from 'clsx'

const ROLE_OPTIONS = [
  { value: 'system', label: '系统', color: 'text-blue-400 bg-blue-500/10 border-blue-500/30' },
  { value: 'user', label: '用户', color: 'text-green-400 bg-green-500/10 border-green-500/30' },
  { value: 'assistant', label: '助手', color: 'text-purple-400 bg-purple-500/10 border-purple-500/30' },
]

const DEFAULT_ENTRY = {
  keys: '', secondary_keys: '', content: '', enabled: true, constant: false,
  priority: 0, injection_position: 0, injection_depth: 4, scan_depth: 0,
  case_sensitive: false, order: 100, role: 'system',
}

export default function WorldBooksPage() {
  const { worldBooks, currentBook, fetchWorldBooks, createWorldBook, deleteWorldBook,
          fetchWorldBook, createEntry, updateEntry, deleteEntry } = useWorldBookStore()
  const { showToast } = useUIStore()

  const { characters, fetchCharacters } = useCharacterStore()

  const [view, setView] = useState('list')
  const [showNewBook, setShowNewBook] = useState(false)
  const [newBookForm, setNewBookForm] = useState({ name: '', description: '', character_id: '' })
  const [showEntryEditor, setShowEntryEditor] = useState(false)
  const [entryForm, setEntryForm] = useState({ ...DEFAULT_ENTRY })
  const [editEntry, setEditEntry] = useState(null)
  const [expandedEntry, setExpandedEntry] = useState(null) // 条目列表中展开详情

  useEffect(() => { fetchWorldBooks(); fetchCharacters() }, [])

  const handleOpenBook = async (id) => {
    await fetchWorldBook(id)
    setView('book')
  }

  const handleCreateBook = async () => {
    if (!newBookForm.name.trim()) { showToast('请填写世界书名称', 'error'); return }
    try {
      await createWorldBook(newBookForm)
      setShowNewBook(false)
      setNewBookForm({ name: '', description: '', character_id: '' })
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

  const openEntryEditor = (entry = null) => {
    if (entry) {
      setEntryForm({ ...DEFAULT_ENTRY, ...entry })
      setEditEntry(entry)
    } else {
      setEntryForm({ ...DEFAULT_ENTRY })
      setEditEntry(null)
    }
    setShowEntryEditor(true)
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
      setShowEntryEditor(false)
      setEditEntry(null)
    } catch { showToast('保存失败', 'error') }
  }

  const handleToggleEntry = async (entry) => {
    try {
      await updateEntry(entry.id, { ...entry, enabled: !entry.enabled })
    } catch { showToast('操作失败', 'error') }
  }

  // 世界书条目详情视图
  if (view === 'book' && currentBook) {
    const entries = currentBook.entries || []
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
            onClick={() => openEntryEditor()}
            className="btn-primary flex items-center gap-1.5 py-2 px-3 text-sm"
          >
            <Plus size={15} /> 添加
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 space-y-2 pb-4">
          {entries.length === 0 ? (
            <EmptyState icon={BookOpen} title="还没有条目" description="添加关键词触发的知识条目" />
          ) : entries.map(entry => (
            <div key={entry.id}
              className={clsx(
                'card overflow-hidden transition-colors',
                !entry.enabled && 'opacity-50'
              )}
            >
              {/* 条目头部摘要 */}
              <div className="flex items-center gap-2 p-3 cursor-pointer hover:bg-surface-hover"
                onClick={() => setExpandedEntry(expandedEntry === entry.id ? null : entry.id)}>
                {/* 常驻标记 */}
                {entry.constant && <Pin size={13} className="text-amber-400 flex-shrink-0" />}

                {/* 角色标签 */}
                <span className={clsx('text-[10px] px-1.5 py-0.5 rounded border font-medium flex-shrink-0',
                  ROLE_OPTIONS.find(r => r.value === entry.role)?.color || ROLE_OPTIONS[0].color
                )}>
                  {ROLE_OPTIONS.find(r => r.value === entry.role)?.label || '系统'}
                </span>

                {/* 关键词 */}
                <div className="flex-1 min-w-0">
                  {entry.keys ? (
                    <div className="flex gap-1 flex-wrap">
                      {entry.keys.split(',').slice(0, 3).map(k => k.trim()).filter(Boolean).map(k => (
                        <span key={k} className="text-[11px] bg-primary-500/15 text-primary-300
                                                  px-1.5 py-0.5 rounded border border-primary-500/20">
                          {k}
                        </span>
                      ))}
                      {entry.keys.split(',').length > 3 && (
                        <span className="text-[10px] text-gray-500">+{entry.keys.split(',').length - 3}</span>
                      )}
                    </div>
                  ) : entry.constant ? (
                    <span className="text-xs text-amber-400">常驻注入</span>
                  ) : (
                    <span className="text-xs text-gray-500">无关键词</span>
                  )}
                </div>

                {/* 深度 */}
                <span className="text-[10px] text-gray-600 flex-shrink-0">
                  D{entry.injection_depth || 0}
                </span>

                {/* 开关 */}
                <button onClick={e => { e.stopPropagation(); handleToggleEntry(entry) }}>
                  {entry.enabled
                    ? <ToggleRight size={18} className="text-primary-400" />
                    : <ToggleLeft size={18} className="text-gray-600" />
                  }
                </button>

                {/* 展开/折叠 */}
                {expandedEntry === entry.id
                  ? <ChevronUp size={16} className="text-gray-500" />
                  : <ChevronDown size={16} className="text-gray-500" />
                }
              </div>

              {/* 展开详情 */}
              {expandedEntry === entry.id && (
                <div className="px-3 pb-3 border-t border-surface-border/50 space-y-2 pt-2">
                  <p className="text-xs text-gray-300 whitespace-pre-wrap line-clamp-4">{entry.content}</p>

                  <div className="flex gap-2 flex-wrap text-[10px] text-gray-500">
                    <span>优先级 {entry.priority}</span>
                    <span>深度 {entry.injection_depth}</span>
                    <span>{entry.injection_position === 1 ? '绝对' : '相对'}位置</span>
                    {entry.scan_depth > 0 && <span>扫描 {entry.scan_depth} 条</span>}
                    {entry.case_sensitive && <span>大小写敏感</span>}
                    {entry.secondary_keys && <span>AND: {entry.secondary_keys}</span>}
                  </div>

                  <div className="flex gap-2 pt-1">
                    <button
                      onClick={() => openEntryEditor(entry)}
                      className="flex-1 py-2 text-xs text-center rounded-lg bg-primary-600/20 text-primary-300
                                 hover:bg-primary-600/30 transition-colors"
                    >
                      编辑
                    </button>
                    <button
                      onClick={() => deleteEntry(entry.id)}
                      className="py-2 px-4 text-xs rounded-lg bg-red-500/10 text-red-400
                                 hover:bg-red-500/20 transition-colors"
                    >
                      删除
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>

        {/* 条目编辑弹窗 */}
        <Modal open={showEntryEditor} onClose={() => { setShowEntryEditor(false); setEditEntry(null) }}
          title={editEntry ? '编辑条目' : '新建条目'}>
          <div className="space-y-4">
            {/* 关键词 */}
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">主关键词（逗号分隔，OR 逻辑）</label>
              <input className="w-full input-base text-sm" value={entryForm.keys}
                onChange={e => setEntryForm(f => ({ ...f, keys: e.target.value }))}
                placeholder="例如：魔法,法师,魔法学院" />
            </div>

            <div>
              <label className="block text-xs text-gray-400 mb-1.5">次关键词（逗号分隔，AND 逻辑，需同时命中）</label>
              <input className="w-full input-base text-sm" value={entryForm.secondary_keys}
                onChange={e => setEntryForm(f => ({ ...f, secondary_keys: e.target.value }))}
                placeholder="留空则只看主关键词" />
            </div>

            {/* 内容 */}
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">注入内容 *</label>
              <textarea className="w-full input-base resize-none text-sm" rows={5}
                value={entryForm.content}
                onChange={e => setEntryForm(f => ({ ...f, content: e.target.value }))}
                placeholder="当关键词命中时，此内容将被注入到上下文中" />
            </div>

            {/* 注入配置 - 第一行 */}
            <div className="grid grid-cols-3 gap-2">
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">角色</label>
                <select className="w-full input-base text-xs py-2 bg-surface appearance-none"
                  value={entryForm.role}
                  onChange={e => setEntryForm(f => ({ ...f, role: e.target.value }))}>
                  {ROLE_OPTIONS.map(r => <option key={r.value} value={r.value}>{r.label}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">注入方式</label>
                <select className="w-full input-base text-xs py-2 bg-surface appearance-none"
                  value={entryForm.injection_position}
                  onChange={e => setEntryForm(f => ({ ...f, injection_position: parseInt(e.target.value) }))}>
                  <option value={0}>相对末尾</option>
                  <option value={1}>绝对位置</option>
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">注入深度</label>
                <input type="number" className="w-full input-base text-xs py-2" min="0"
                  value={entryForm.injection_depth}
                  onChange={e => setEntryForm(f => ({ ...f, injection_depth: parseInt(e.target.value) || 0 }))} />
              </div>
            </div>

            {/* 注入配置 - 第二行 */}
            <div className="grid grid-cols-3 gap-2">
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">扫描深度</label>
                <input type="number" className="w-full input-base text-xs py-2" min="0"
                  value={entryForm.scan_depth}
                  onChange={e => setEntryForm(f => ({ ...f, scan_depth: parseInt(e.target.value) || 0 }))} />
                <p className="text-[9px] text-gray-600 mt-0.5">0=全部消息</p>
              </div>
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">优先级</label>
                <input type="number" className="w-full input-base text-xs py-2"
                  value={entryForm.priority}
                  onChange={e => setEntryForm(f => ({ ...f, priority: parseInt(e.target.value) || 0 }))} />
              </div>
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">排序</label>
                <input type="number" className="w-full input-base text-xs py-2"
                  value={entryForm.order}
                  onChange={e => setEntryForm(f => ({ ...f, order: parseInt(e.target.value) || 0 }))} />
              </div>
            </div>

            {/* 开关选项 */}
            <div className="flex gap-2 flex-wrap">
              {[
                { key: 'enabled', label: '启用' },
                { key: 'constant', label: '常驻（无需关键词）' },
                { key: 'case_sensitive', label: '大小写敏感' },
              ].map(({ key, label }) => (
                <label key={key} className={clsx(
                  'flex items-center gap-2 cursor-pointer px-3 py-2 rounded-lg border text-xs transition-colors',
                  entryForm[key]
                    ? 'border-primary-500/40 bg-primary-500/10 text-primary-300'
                    : 'border-surface-border text-gray-500 hover:bg-surface-hover'
                )}>
                  <input type="checkbox" checked={entryForm[key]}
                    onChange={e => setEntryForm(f => ({ ...f, [key]: e.target.checked }))}
                    className="hidden" />
                  <span>{label}</span>
                </label>
              ))}
            </div>

            {/* 说明 */}
            <div className="text-[10px] text-gray-600 bg-surface/50 p-3 rounded-lg border border-surface-border leading-relaxed">
              <p><b>主关键词</b>: OR 逻辑，任一命中即触发</p>
              <p><b>次关键词</b>: AND 逻辑，需与主关键词同时命中</p>
              <p><b>深度0</b>=紧跟系统提示词 | <b>深度N(相对)</b>=从末尾倒数第N条 | <b>深度N(绝对)</b>=第N条后</p>
              <p><b>常驻</b>: 无需关键词触发，每次对话都注入</p>
              <p><b>扫描深度</b>: 往回扫描几条消息寻找关键词，0=扫描全部</p>
            </div>

            <div className="flex gap-3 pt-2">
              <button onClick={() => { setShowEntryEditor(false); setEditEntry(null) }}
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
            <div className={clsx(
              'w-11 h-11 rounded-2xl flex items-center justify-center flex-shrink-0 border',
              wb.character_id
                ? 'bg-gradient-to-br from-purple-500/20 to-pink-500/20 border-purple-500/20'
                : 'bg-gradient-to-br from-amber-500/20 to-orange-500/20 border-amber-500/20'
            )}>
              {wb.character_id
                ? <User size={20} className="text-purple-400" />
                : <Globe size={20} className="text-amber-400" />
              }
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-0.5">
                <p className="font-semibold text-sm">{wb.name}</p>
                <span className={clsx('text-[10px] px-1.5 py-0.5 rounded-full border',
                  wb.character_id
                    ? 'bg-purple-500/15 text-purple-300 border-purple-500/30'
                    : 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                )}>
                  {wb.character_id ? (wb.character_name || '角色') : '全局'}
                </span>
              </div>
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
          {/* 类型选择 */}
          <div>
            <label className="block text-xs text-gray-400 mb-2">类型</label>
            <div className="flex gap-3">
              <button
                onClick={() => setNewBookForm(f => ({ ...f, character_id: '' }))}
                className={clsx('flex-1 flex items-center justify-center gap-2 py-3 rounded-xl border transition-all',
                  !newBookForm.character_id
                    ? 'border-amber-500/50 bg-amber-500/10 text-amber-300'
                    : 'border-surface-border text-gray-400 hover:bg-surface-hover'
                )}>
                <Globe size={16} />
                <span className="text-sm">全局</span>
              </button>
              <button
                onClick={() => setNewBookForm(f => ({ ...f, character_id: characters[0]?.id || '' }))}
                className={clsx('flex-1 flex items-center justify-center gap-2 py-3 rounded-xl border transition-all',
                  newBookForm.character_id
                    ? 'border-purple-500/50 bg-purple-500/10 text-purple-300'
                    : 'border-surface-border text-gray-400 hover:bg-surface-hover'
                )}>
                <User size={16} />
                <span className="text-sm">绑定角色</span>
              </button>
            </div>
          </div>

          {/* 角色选择（仅绑定模式） */}
          {newBookForm.character_id !== '' && (
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">选择角色</label>
              <select className="w-full input-base text-sm bg-surface appearance-none"
                value={newBookForm.character_id}
                onChange={e => setNewBookForm(f => ({ ...f, character_id: e.target.value }))}>
                {characters.map(c => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            </div>
          )}

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
