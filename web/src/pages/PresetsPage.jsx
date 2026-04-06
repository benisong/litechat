import React, { useEffect, useState, useRef } from 'react'
import { Layers, Plus, Trash2, Edit2, Check, ChevronDown, ChevronUp,
         Upload, ToggleLeft, ToggleRight, GripVertical, Copy } from 'lucide-react'
import { usePresetStore, useUIStore, useAuthStore } from '../store'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'
import clsx from 'clsx'

// 默认简单模式模板
const DEFAULT_SYSTEM_PROMPT = '你是{{char}}。请根据角色设定进行扮演，保持角色一致性。\n\n角色描述：{{description}}\n\n性格：{{personality}}\n\n场景：{{scenario}}'

// 新建空的提示词条目
function newEntry(overrides = {}) {
  return {
    id: 'entry-' + Date.now() + '-' + Math.random().toString(36).slice(2, 6),
    name: '新提示词',
    content: '',
    role: 'system',
    enabled: true,
    injection_position: 0,
    injection_depth: 0,
    order: 100,
    ...overrides,
  }
}

// 默认高级模式模板
const DEFAULT_PROMPTS = [
  newEntry({ id: 'main', name: '主提示词', content: '你是{{char}}，请根据角色设定进行扮演。保持角色一致性，不要打破角色。', role: 'system', injection_depth: 0, order: 0 }),
  newEntry({ id: 'char-desc', name: '角色描述', content: '角色描述：{{description}}\n性格：{{personality}}', role: 'system', injection_depth: 0, order: 10 }),
  newEntry({ id: 'scenario', name: '场景', content: '当前场景：{{scenario}}', role: 'system', injection_depth: 0, order: 20 }),
  newEntry({ id: 'authors-note', name: '作者注释', content: '[请保持角色扮演，用详细的叙述方式回应]', role: 'system', injection_position: 0, injection_depth: 2, order: 100 }),
]

const DEFAULT_FORM = {
  name: '',
  system_prompt: DEFAULT_SYSTEM_PROMPT,
  prompts: '',
  temperature: 0.8,
  max_tokens: 2048,
  top_p: 0.9,
  is_default: false,
}

const ROLE_OPTIONS = [
  { value: 'system', label: '系统', color: 'text-blue-400 bg-blue-500/10 border-blue-500/30' },
  { value: 'user', label: '用户', color: 'text-green-400 bg-green-500/10 border-green-500/30' },
  { value: 'assistant', label: '助手', color: 'text-purple-400 bg-purple-500/10 border-purple-500/30' },
]

export default function PresetsPage() {
  const { presets, fetchPresets, createPreset, updatePreset, deletePreset } = usePresetStore()
  const { showToast } = useUIStore()
  const [editPreset, setEditPreset] = useState(null) // null | 'new' | preset obj
  const [form, setForm] = useState(DEFAULT_FORM)
  const [mode, setMode] = useState('simple') // 'simple' | 'advanced'
  const [entries, setEntries] = useState([]) // 高级模式的多段提示词
  const [expandedEntry, setExpandedEntry] = useState(null) // 展开编辑的条目 ID
  const fileInputRef = useRef(null)
  const isAdmin = useAuthStore(s => s.user?.role === 'admin')

  useEffect(() => { fetchPresets() }, [])

  const openNew = () => {
    setForm(DEFAULT_FORM)
    setEntries([])
    setMode('simple')
    setEditPreset('new')
  }

  const openEdit = (preset) => {
    setForm({ ...preset })
    // 如果有多段提示词，切换到高级模式
    if (preset.prompts) {
      try {
        setEntries(JSON.parse(preset.prompts))
        setMode('advanced')
      } catch {
        setEntries([])
        setMode('simple')
      }
    } else {
      setEntries([])
      setMode('simple')
    }
    setEditPreset(preset)
  }

  const handleSave = async () => {
    if (!form.name.trim()) { showToast('请填写预设名称', 'error'); return }
    const saveData = { ...form }
    // 高级模式：将 entries 序列化为 JSON
    if (mode === 'advanced' && entries.length > 0) {
      saveData.prompts = JSON.stringify(entries)
      saveData.system_prompt = '' // 高级模式不用简单提示词
    } else {
      saveData.prompts = ''
    }
    try {
      if (editPreset === 'new') {
        await createPreset(saveData)
        showToast('预设创建成功', 'success')
      } else {
        await updatePreset(editPreset.id, saveData)
        showToast('保存成功', 'success')
      }
      setEditPreset(null)
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    }
  }

  const handleDelete = async (id, e) => {
    e.stopPropagation()
    try {
      await deletePreset(id)
      showToast('预设已删除', 'success')
    } catch { showToast('删除失败', 'error') }
  }

  // 导入 SillyTavern JSON
  const handleImport = (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => {
      try {
        const json = JSON.parse(ev.target.result)
        // 解析 SillyTavern 格式
        const imported = parseSillyTavernPreset(json)
        setForm(f => ({
          ...f,
          name: imported.name || f.name || file.name.replace('.json', ''),
          temperature: imported.temperature ?? f.temperature,
          max_tokens: imported.max_tokens ?? f.max_tokens,
          top_p: imported.top_p ?? f.top_p,
        }))
        setEntries(imported.entries)
        setMode('advanced')
        showToast(`导入成功：${imported.entries.length} 段提示词`, 'success')
      } catch (err) {
        showToast('导入失败：' + err.message, 'error')
      }
    }
    reader.readAsText(file)
    e.target.value = '' // 重置
  }

  // 解析 SillyTavern 预设 JSON
  function parseSillyTavernPreset(json) {
    const result = {
      name: json.name || '',
      temperature: json.temperature,
      max_tokens: json.max_tokens,
      top_p: json.top_p,
      entries: [],
    }

    // 构建 prompt_order 的 enabled 映射（ST 用 prompt_order 决定实际启用状态和顺序）
    const orderMap = {} // identifier → { enabled, index }
    if (Array.isArray(json.prompt_order)) {
      // 取最后一个 character_id 的 order（通常是实际使用的）
      const lastOrder = json.prompt_order[json.prompt_order.length - 1]
      if (lastOrder?.order) {
        lastOrder.order.forEach((item, idx) => {
          orderMap[item.identifier] = { enabled: item.enabled !== false, index: idx }
        })
      }
    }

    // SillyTavern 格式：prompts 数组
    if (Array.isArray(json.prompts)) {
      result.entries = json.prompts
        .filter(p => (p.prompt || p.content) && !p.marker) // 跳过空条目和 marker
        .map((p, i) => {
          const id = p.identifier || p.id || `imported-${i}`
          // 优先用 prompt_order 的 enabled 状态
          const orderInfo = orderMap[id]
          const enabled = orderInfo !== undefined ? orderInfo.enabled : (p.enabled !== false)
          const sortIndex = orderInfo !== undefined ? orderInfo.index : i

          return {
            id,
            name: p.name || p.display_name || `提示词 ${i + 1}`,
            content: p.prompt || p.content || '',
            role: p.role || 'system',
            enabled,
            system_prompt: p.system_prompt !== false,
            injection_position: p.injection_position ?? 0,
            injection_depth: p.injection_depth ?? 0,
            order: p.injection_order ?? (sortIndex * 10),
          }
        })
    }
    // 兼容：如果没有 prompts 数组但有 system_prompt
    if (result.entries.length === 0 && json.system_prompt) {
      result.entries = [newEntry({ name: '系统提示词', content: json.system_prompt })]
    }

    return result
  }

  // 条目操作
  const addEntry = () => {
    const entry = newEntry()
    setEntries(prev => [...prev, entry])
    setExpandedEntry(entry.id)
  }

  const removeEntry = (id) => {
    setEntries(prev => prev.filter(e => e.id !== id))
    if (expandedEntry === id) setExpandedEntry(null)
  }

  const updateEntry = (id, field, value) => {
    setEntries(prev => prev.map(e => e.id === id ? { ...e, [field]: value } : e))
  }

  const toggleEntry = (id) => {
    setEntries(prev => prev.map(e => e.id === id ? { ...e, enabled: !e.enabled } : e))
  }

  const moveEntry = (id, dir) => {
    setEntries(prev => {
      const idx = prev.findIndex(e => e.id === id)
      if (idx < 0) return prev
      const newIdx = idx + dir
      if (newIdx < 0 || newIdx >= prev.length) return prev
      const copy = [...prev]
      ;[copy[idx], copy[newIdx]] = [copy[newIdx], copy[idx]]
      return copy
    })
  }

  // 切换到高级模式时，从简单模式的文本生成初始 entries
  const switchToAdvanced = () => {
    if (entries.length === 0) {
      if (form.system_prompt) {
        setEntries([newEntry({ name: '系统提示词', content: form.system_prompt, order: 0 })])
      } else {
        setEntries([...DEFAULT_PROMPTS])
      }
    }
    setMode('advanced')
  }

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 pt-12 pb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold">预设</h1>
        <button onClick={openNew} className="btn-primary flex items-center gap-2 py-2 px-4 text-sm">
          <Plus size={16} /> 新建
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-4 space-y-2">
        {presets.length === 0 ? (
          <EmptyState icon={Layers} title="还没有预设" description="创建提示词预设，快速切换聊天风格" />
        ) : presets.map(preset => (
          <div
            key={preset.id}
            className="card p-4 cursor-pointer hover:bg-surface-hover transition-colors"
            onClick={() => openEdit(preset)}
          >
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <span className="font-semibold text-sm">{preset.name}</span>
                {preset.is_default && (
                  <span className="flex items-center gap-1 text-[10px] bg-primary-500/20 text-primary-300
                                   px-2 py-0.5 rounded-full border border-primary-500/20">
                    <Check size={10} /> 默认
                  </span>
                )}
                {preset.prompts && (
                  <span className="text-[10px] bg-amber-500/20 text-amber-300
                                   px-2 py-0.5 rounded-full border border-amber-500/20">
                    多段
                  </span>
                )}
              </div>
              <div className="flex items-center gap-1">
                <button onClick={e => { e.stopPropagation(); openEdit(preset) }}
                  className="p-2 text-gray-500 hover:text-white transition-colors rounded-lg">
                  <Edit2 size={14} />
                </button>
                <button onClick={e => handleDelete(preset.id, e)}
                  className="p-2 text-gray-500 hover:text-red-400 transition-colors rounded-lg">
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
            <p className="text-xs text-gray-500 line-clamp-2">
              {preset.prompts
                ? `${JSON.parse(preset.prompts).filter(p => p.enabled !== false).length} 段提示词`
                : preset.system_prompt
              }
            </p>
            <div className="flex gap-3 mt-2 text-[11px] text-gray-600">
              <span>温度 {preset.temperature}</span>
              <span>MaxTokens {preset.max_tokens}</span>
              <span>Top-P {preset.top_p}</span>
            </div>
          </div>
        ))}
      </div>

      {/* 编辑弹窗 */}
      <Modal
        open={!!editPreset}
        onClose={() => setEditPreset(null)}
        title={editPreset === 'new' ? '新建预设' : '编辑预设'}
      >
        <div className="space-y-4">
          {/* 名称 */}
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">预设名称 *</label>
            <input className="w-full input-base" value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              placeholder="例如：角色扮演标准" />
          </div>

          {/* 模式切换 + 导入 */}
          <div className="flex items-center gap-2">
            <div className="flex bg-dark-200 rounded-xl p-0.5 border border-surface-border">
              <button
                onClick={() => setMode('simple')}
                className={clsx('px-3 py-1.5 rounded-lg text-xs font-medium transition-colors',
                  mode === 'simple' ? 'bg-primary-600 text-white' : 'text-gray-400 hover:text-white'
                )}
              >
                简单模式
              </button>
              <button
                onClick={switchToAdvanced}
                className={clsx('px-3 py-1.5 rounded-lg text-xs font-medium transition-colors',
                  mode === 'advanced' ? 'bg-primary-600 text-white' : 'text-gray-400 hover:text-white'
                )}
              >
                高级模式
              </button>
            </div>
            {/* 仅管理员可导入 SillyTavern 预设 */}
            {isAdmin && (
              <>
                <button
                  onClick={() => fileInputRef.current?.click()}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg border
                             border-amber-500/40 text-amber-300 hover:bg-amber-500/10 transition-colors ml-auto"
                >
                  <Upload size={12} /> 导入 ST
                </button>
                <input ref={fileInputRef} type="file" accept=".json" className="hidden" onChange={handleImport} />
              </>
            )}
          </div>

          {/* 简单模式 */}
          {mode === 'simple' && (
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">系统提示词</label>
              <textarea className="w-full input-base resize-none text-sm" rows={6}
                value={form.system_prompt}
                onChange={e => setForm(f => ({ ...f, system_prompt: e.target.value }))}
                placeholder="系统提示词…" />
            </div>
          )}

          {/* 高级模式：多段提示词 */}
          {mode === 'advanced' && (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-400">
                  {entries.length} 段提示词（{entries.filter(e => e.enabled).length} 启用）
                </span>
                <button onClick={addEntry}
                  className="flex items-center gap-1 text-xs text-primary-400 hover:text-primary-300">
                  <Plus size={13} /> 添加
                </button>
              </div>

              <div className="space-y-1.5 max-h-[50vh] overflow-y-auto">
                {entries.map((entry, idx) => (
                  <div key={entry.id}
                    className={clsx(
                      'rounded-xl border transition-colors',
                      entry.enabled ? 'border-surface-border bg-surface' : 'border-surface-border/50 bg-dark-200 opacity-60'
                    )}
                  >
                    {/* 条目头部 */}
                    <div className="flex items-center gap-2 px-3 py-2 cursor-pointer"
                      onClick={() => setExpandedEntry(expandedEntry === entry.id ? null : entry.id)}>
                      {/* 排序按钮 */}
                      <div className="flex flex-col -my-1">
                        <button onClick={e => { e.stopPropagation(); moveEntry(entry.id, -1) }}
                          className="text-gray-600 hover:text-gray-300 p-0.5">
                          <ChevronUp size={12} />
                        </button>
                        <button onClick={e => { e.stopPropagation(); moveEntry(entry.id, 1) }}
                          className="text-gray-600 hover:text-gray-300 p-0.5">
                          <ChevronDown size={12} />
                        </button>
                      </div>

                      {/* 角色标签 */}
                      <span className={clsx('text-[10px] px-1.5 py-0.5 rounded border font-medium',
                        ROLE_OPTIONS.find(r => r.value === entry.role)?.color || ROLE_OPTIONS[0].color
                      )}>
                        {ROLE_OPTIONS.find(r => r.value === entry.role)?.label || '系统'}
                      </span>

                      {/* 名称 */}
                      <span className="text-sm font-medium flex-1 truncate">{entry.name}</span>

                      {/* 深度标记 */}
                      {entry.injection_depth > 0 && (
                        <span className="text-[10px] text-gray-500">
                          深度{entry.injection_depth}
                        </span>
                      )}

                      {/* 启用开关 */}
                      <button onClick={e => { e.stopPropagation(); toggleEntry(entry.id) }}>
                        {entry.enabled
                          ? <ToggleRight size={18} className="text-primary-400" />
                          : <ToggleLeft size={18} className="text-gray-600" />
                        }
                      </button>

                      {/* 删除 */}
                      <button onClick={e => { e.stopPropagation(); removeEntry(entry.id) }}
                        className="text-gray-600 hover:text-red-400 transition-colors p-1">
                        <Trash2 size={13} />
                      </button>
                    </div>

                    {/* 展开编辑 */}
                    {expandedEntry === entry.id && (
                      <div className="px-3 pb-3 pt-1 space-y-3 border-t border-surface-border/50">
                        <div className="grid grid-cols-2 gap-2">
                          <div>
                            <label className="block text-[10px] text-gray-500 mb-1">名称</label>
                            <input className="w-full input-base text-xs py-2"
                              value={entry.name}
                              onChange={e => updateEntry(entry.id, 'name', e.target.value)} />
                          </div>
                          <div>
                            <label className="block text-[10px] text-gray-500 mb-1">角色</label>
                            <select className="w-full input-base text-xs py-2 bg-surface appearance-none"
                              value={entry.role}
                              onChange={e => updateEntry(entry.id, 'role', e.target.value)}>
                              {ROLE_OPTIONS.map(r => (
                                <option key={r.value} value={r.value}>{r.label}</option>
                              ))}
                            </select>
                          </div>
                        </div>

                        <div>
                          <label className="block text-[10px] text-gray-500 mb-1">内容</label>
                          <textarea className="w-full input-base text-xs resize-none py-2" rows={4}
                            value={entry.content}
                            onChange={e => updateEntry(entry.id, 'content', e.target.value)}
                            placeholder="支持变量：{{char}} {{description}} {{personality}} {{scenario}}" />
                        </div>

                        <div className="grid grid-cols-3 gap-2">
                          <div>
                            <label className="block text-[10px] text-gray-500 mb-1">注入方式</label>
                            <select className="w-full input-base text-xs py-2 bg-surface appearance-none"
                              value={entry.injection_position}
                              onChange={e => updateEntry(entry.id, 'injection_position', parseInt(e.target.value))}>
                              <option value={0}>相对末尾</option>
                              <option value={1}>绝对位置</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-[10px] text-gray-500 mb-1">注入深度</label>
                            <input type="number" className="w-full input-base text-xs py-2" min="0"
                              value={entry.injection_depth}
                              onChange={e => updateEntry(entry.id, 'injection_depth', parseInt(e.target.value) || 0)} />
                          </div>
                          <div>
                            <label className="block text-[10px] text-gray-500 mb-1">排序</label>
                            <input type="number" className="w-full input-base text-xs py-2"
                              value={entry.order}
                              onChange={e => updateEntry(entry.id, 'order', parseInt(e.target.value) || 0)} />
                          </div>
                        </div>

                        <p className="text-[10px] text-gray-600 leading-relaxed">
                          深度0=在聊天历史前 | 深度N(相对)=从末尾倒数第N条插入 | 深度N(绝对)=第N条后插入
                        </p>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* 参数 */}
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">温度</label>
              <input type="number" className="w-full input-base text-sm" step="0.1" min="0" max="2"
                value={form.temperature}
                onChange={e => setForm(f => ({ ...f, temperature: parseFloat(e.target.value) || 0.8 }))} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">MaxTokens</label>
              <input type="number" className="w-full input-base text-sm" step="256" min="256"
                value={form.max_tokens}
                onChange={e => setForm(f => ({ ...f, max_tokens: parseInt(e.target.value) || 2048 }))} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">Top-P</label>
              <input type="number" className="w-full input-base text-sm" step="0.1" min="0" max="1"
                value={form.top_p}
                onChange={e => setForm(f => ({ ...f, top_p: parseFloat(e.target.value) || 0.9 }))} />
            </div>
          </div>

          <label className="flex items-center gap-3 cursor-pointer p-3 rounded-xl
                             bg-surface border border-surface-border hover:bg-surface-hover">
            <input type="checkbox" checked={form.is_default}
              onChange={e => setForm(f => ({ ...f, is_default: e.target.checked }))}
              className="w-4 h-4 rounded accent-violet-500" />
            <span className="text-sm">设为默认预设</span>
          </label>

          <div className="flex gap-3 pt-2">
            <button onClick={() => setEditPreset(null)}
              className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                         hover:bg-surface-hover transition-colors">
              取消
            </button>
            <button onClick={handleSave} className="flex-1 btn-primary py-3">保存</button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
