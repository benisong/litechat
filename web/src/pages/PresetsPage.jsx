import React, { useEffect, useRef, useState } from 'react'
import {
  Layers,
  Plus,
  Trash2,
  Edit2,
  Check,
  ChevronDown,
  ChevronUp,
  Upload,
  ToggleLeft,
  ToggleRight,
  Database,
} from 'lucide-react'
import { usePresetStore, useUIStore, useAuthStore, useSettingsStore } from '../store'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'
import clsx from 'clsx'

function newEntry(overrides = {}) {
  return {
    id: 'entry-' + Date.now() + '-' + Math.random().toString(36).slice(2, 6),
    name: '新提示词',
    content: '',
    role: 'system',
    enabled: true,
    system_prompt: true,
    injection_position: 0,
    injection_depth: 0,
    order: 100,
    ...overrides,
  }
}

const DEFAULT_PROMPTS = [
  newEntry({
    id: 'main',
    name: '主提示词',
    content: '你是{{char}}，请根据角色设定进行扮演，保持角色一致性，不要打破角色。',
    role: 'system',
    system_prompt: true,
    injection_depth: 0,
    order: 0,
  }),
  newEntry({
    id: 'char-desc',
    name: '角色描述',
    content: '角色描述：{{description}}\n性格：{{personality}}',
    role: 'system',
    system_prompt: true,
    injection_depth: 0,
    order: 10,
  }),
  newEntry({
    id: 'scenario',
    name: '场景',
    content: '当前场景：{{scenario}}',
    role: 'system',
    system_prompt: true,
    injection_depth: 0,
    order: 20,
  }),
  newEntry({
    id: 'authors-note',
    name: '作者注记',
    content: '[请保持角色扮演，用更细致、自然的方式回应]',
    role: 'system',
    system_prompt: false,
    injection_position: 0,
    injection_depth: 2,
    order: 100,
  }),
]

const DEFAULT_FORM = {
  name: '',
  system_prompt: '',
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

const MEMORY_PROTOCOL_PREVIEW = `<chat_summary>
<plot>...</plot>
<relationship>...</relationship>
<user_facts>...</user_facts>
<world_state>...</world_state>
<open_loops>...</open_loops>
</chat_summary>`

function parsePresetEntries(preset) {
  if (preset.prompts) {
    try {
      return JSON.parse(preset.prompts)
    } catch {
      return [...DEFAULT_PROMPTS]
    }
  }
  if (preset.system_prompt) {
    return [newEntry({ name: '系统提示词', content: preset.system_prompt, order: 0 })]
  }
  return [...DEFAULT_PROMPTS]
}

function parseSillyTavernPreset(json) {
  const result = {
    name: json.name || '',
    temperature: json.temperature,
    max_tokens: json.max_tokens,
    top_p: json.top_p,
    entries: [],
  }

  const orderMap = {}
  if (Array.isArray(json.prompt_order)) {
    for (const promptOrder of json.prompt_order) {
      if (!Array.isArray(promptOrder?.order)) continue
      promptOrder.order.forEach((item, index) => {
        orderMap[item.identifier] = {
          enabled: item.enabled === true,
          index,
        }
      })
    }
  }

  if (Array.isArray(json.prompts)) {
    result.entries = json.prompts
      .filter(prompt => (prompt.prompt || prompt.content) && !prompt.marker)
      .map((prompt, index) => {
        const id = prompt.identifier || prompt.id || `imported-${index}`
        const orderInfo = orderMap[id]
        return {
          id,
          name: prompt.name || prompt.display_name || `提示词 ${index + 1}`,
          content: prompt.prompt || prompt.content || '',
          role: prompt.role || 'system',
          enabled: orderInfo ? orderInfo.enabled : prompt.enabled !== false,
          system_prompt: prompt.system_prompt !== false,
          injection_position: prompt.injection_position ?? 0,
          injection_depth: prompt.injection_depth ?? 0,
          order: orderInfo ? orderInfo.index : index,
        }
      })
  }

  if (result.entries.length === 0 && json.system_prompt) {
    result.entries = [newEntry({ name: '系统提示词', content: json.system_prompt })]
  }

  return result
}

function getEnabledEntryCount(preset) {
  if (!preset.prompts) {
    return preset.system_prompt ? 1 : 0
  }
  try {
    return JSON.parse(preset.prompts).filter(entry => entry.enabled !== false).length
  } catch {
    return 0
  }
}

export default function PresetsPage() {
  const { presets, fetchPresets, createPreset, updatePreset, deletePreset } = usePresetStore()
  const { settings, fetchSettings, saveSettings } = useSettingsStore()
  const { showToast } = useUIStore()
  const [editPreset, setEditPreset] = useState(null)
  const [form, setForm] = useState(DEFAULT_FORM)
  const [entries, setEntries] = useState([])
  const [expandedEntry, setExpandedEntry] = useState(null)
  const [activeTab, setActiveTab] = useState('presets')
  const [memoryPrompt, setMemoryPrompt] = useState('')
  const [savingMemory, setSavingMemory] = useState(false)
  const fileInputRef = useRef(null)
  const isAdmin = useAuthStore(state => state.user?.role === 'admin')
  const isServiceMode = settings.service_mode === 'service'
  const showMemoryTab = isAdmin && isServiceMode

  useEffect(() => {
    fetchPresets()
    if (isAdmin) fetchSettings().catch(() => {})
  }, [])

  useEffect(() => {
    setMemoryPrompt(settings.memory_prompt_suffix || '')
  }, [settings.memory_prompt_suffix])

  useEffect(() => {
    if (!showMemoryTab && activeTab === 'memory') {
      setActiveTab('presets')
    }
  }, [showMemoryTab, activeTab])

  const openNew = () => {
    setForm(DEFAULT_FORM)
    setEntries([...DEFAULT_PROMPTS])
    setExpandedEntry(null)
    setEditPreset('new')
  }

  const openEdit = preset => {
    setForm({ ...preset })
    setEntries(parsePresetEntries(preset))
    setExpandedEntry(null)
    setEditPreset(preset)
  }

  const handleSave = async () => {
    if (!form.name.trim()) {
      showToast('请填写预设名称', 'error')
      return
    }

    const saveData = {
      ...form,
      prompts: entries.length > 0 ? JSON.stringify(entries) : '',
      system_prompt: '',
    }

    try {
      if (editPreset === 'new') {
        await createPreset(saveData)
        showToast('预设创建成功', 'success')
      } else {
        await updatePreset(editPreset.id, saveData)
        showToast('预设保存成功', 'success')
      }
      setEditPreset(null)
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    }
  }

  const handleSaveMemoryPrompt = async () => {
    setSavingMemory(true)
    try {
      await saveSettings({
        ...settings,
        memory_prompt_suffix: memoryPrompt,
      })
      showToast('记忆存储提示词已保存', 'success')
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    } finally {
      setSavingMemory(false)
    }
  }

  const handleDelete = async (id, event) => {
    event.stopPropagation()
    try {
      await deletePreset(id)
      showToast('预设已删除', 'success')
    } catch {
      showToast('删除失败', 'error')
    }
  }

  const handleImport = event => {
    const file = event.target.files?.[0]
    if (!file) return

    const reader = new FileReader()
    reader.onload = loadEvent => {
      try {
        const json = JSON.parse(loadEvent.target.result)
        const imported = parseSillyTavernPreset(json)
        setForm(current => ({
          ...current,
          name: imported.name || current.name || file.name.replace('.json', ''),
          temperature: imported.temperature ?? current.temperature,
          max_tokens: imported.max_tokens ?? current.max_tokens,
          top_p: imported.top_p ?? current.top_p,
        }))
        setEntries(imported.entries)
        showToast(`导入成功，共 ${imported.entries.length} 段提示词`, 'success')
      } catch (err) {
        showToast(`导入失败：${err.message}`, 'error')
      }
    }
    reader.readAsText(file)
    event.target.value = ''
  }

  const addEntry = () => {
    const entry = newEntry()
    setEntries(current => [...current, entry])
    setExpandedEntry(entry.id)
  }

  const removeEntry = id => {
    setEntries(current => current.filter(entry => entry.id !== id))
    if (expandedEntry === id) setExpandedEntry(null)
  }

  const updateEntry = (id, field, value) => {
    setEntries(current => current.map(entry => entry.id === id ? { ...entry, [field]: value } : entry))
  }

  const toggleEntry = id => {
    setEntries(current => current.map(entry => entry.id === id ? { ...entry, enabled: !entry.enabled } : entry))
  }

  const moveEntry = (id, direction) => {
    setEntries(current => {
      const index = current.findIndex(entry => entry.id === id)
      if (index < 0) return current
      const newIndex = index + direction
      if (newIndex < 0 || newIndex >= current.length) return current
      const copy = [...current]
      ;[copy[index], copy[newIndex]] = [copy[newIndex], copy[index]]
      return copy.map((entry, idx) => ({ ...entry, order: idx }))
    })
  }

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 pt-12 pb-4 flex items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">预设</h1>
        {activeTab === 'presets' && (
          <button onClick={openNew} className="btn-primary flex items-center gap-2 py-2 px-4 text-sm">
            <Plus size={16} /> 新建
          </button>
        )}
      </div>

      <div className="px-4 pb-3">
        <div className="inline-flex rounded-xl border border-surface-border bg-surface/70 p-1">
          <button
            onClick={() => setActiveTab('presets')}
            className={clsx(
              'px-3 py-1.5 text-sm rounded-lg transition-colors',
              activeTab === 'presets' ? 'bg-primary-500/15 text-primary-300' : 'text-gray-400 hover:text-gray-200'
            )}
          >
            聊天预设
          </button>
          {showMemoryTab && (
            <button
              onClick={() => setActiveTab('memory')}
              className={clsx(
                'px-3 py-1.5 text-sm rounded-lg transition-colors flex items-center gap-1.5',
                activeTab === 'memory' ? 'bg-primary-500/15 text-primary-300' : 'text-gray-400 hover:text-gray-200'
              )}
            >
              <Database size={14} />
              记忆存储
            </button>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-4 space-y-4 pb-4">
        {activeTab === 'presets' && (
          <>
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
                      <span className="flex items-center gap-1 text-[10px] bg-primary-500/20 text-primary-300 px-2 py-0.5 rounded-full border border-primary-500/20">
                        <Check size={10} /> 默认
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-1">
                    <button onClick={event => { event.stopPropagation(); openEdit(preset) }} className="p-2 text-gray-500 hover:text-white transition-colors rounded-lg">
                      <Edit2 size={14} />
                    </button>
                    <button onClick={event => handleDelete(preset.id, event)} className="p-2 text-gray-500 hover:text-red-400 transition-colors rounded-lg">
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
                <p className="text-xs text-gray-500 line-clamp-2">
                  {preset.prompts ? `${getEnabledEntryCount(preset)} 段提示词` : preset.system_prompt || '暂无提示词'}
                </p>
                <div className="flex gap-3 mt-2 text-[11px] text-gray-600">
                  <span>温度 {preset.temperature}</span>
                  <span>MaxTokens {preset.max_tokens}</span>
                  <span>Top-P {preset.top_p}</span>
                </div>
              </div>
            ))}
          </>
        )}

        {activeTab === 'memory' && showMemoryTab && (
          <>
            <div className="card p-4 space-y-3">
              <div className="flex items-center gap-2 text-primary-300">
                <Database size={16} />
                <h2 className="font-semibold">记忆存储提示词</h2>
              </div>
              <p className="text-sm text-gray-300">
                这套提示词只在 <code>service</code> 模式下生效，用于后台异步生成小摘要和大摘要。
                输出协议、标签结构和安全约束由系统固定，这里只开放补充提示词给管理员微调。
              </p>
              <p className="text-xs text-primary-300">
                摘要模型请在“设置 / API 配置”中单独选择；默认会跟随当前聊天模型。
              </p>
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-amber-200">
                不要要求模型改动标签结构、输出 Markdown、增加额外字段，或把隐藏思考内容写进摘要。
              </div>
            </div>

            <div className="card p-4 space-y-3">
              <label className="block text-xs text-gray-400">补充提示词</label>
              <textarea
                className="w-full input-base resize-none text-sm"
                rows={10}
                value={memoryPrompt}
                onChange={event => setMemoryPrompt(event.target.value)}
                placeholder="例如：更重视关系推进、未完成事项和用户明确表达过的偏好；重复寒暄可以强压缩。"
              />
              <p className="text-xs text-gray-500">
                系统会把这段内容拼接到固定摘要骨架后面，用于调整摘要关注点，不会替换底层解析协议。
              </p>
              <button
                onClick={handleSaveMemoryPrompt}
                disabled={savingMemory}
                className="btn-primary py-2.5 px-4 text-sm"
              >
                {savingMemory ? '保存中...' : '保存记忆存储提示词'}
              </button>
            </div>

            <div className="card p-4 space-y-3">
              <label className="block text-xs text-gray-400">固定输出协议预览</label>
              <pre className="rounded-xl border border-surface-border bg-dark-200 p-3 text-xs text-gray-300 whitespace-pre-wrap">
                {MEMORY_PROTOCOL_PREVIEW}
              </pre>
            </div>
          </>
        )}
      </div>

      <Modal
        open={!!editPreset}
        onClose={() => setEditPreset(null)}
        title={editPreset === 'new' ? '新建预设' : '编辑预设'}
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">预设名称 *</label>
            <input
              className="w-full input-base"
              value={form.name}
              onChange={event => setForm(current => ({ ...current, name: event.target.value }))}
              placeholder="例如：角色扮演标准版"
            />
          </div>

          {isAdmin && (
            <div className="flex items-center gap-2">
              <button
                onClick={() => fileInputRef.current?.click()}
                className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg border border-amber-500/40 text-amber-300 hover:bg-amber-500/10 transition-colors ml-auto"
              >
                <Upload size={12} /> 导入 ST
              </button>
              <input ref={fileInputRef} type="file" accept=".json" className="hidden" onChange={handleImport} />
            </div>
          )}

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-xs text-gray-400">
                {entries.length} 段提示词，{entries.filter(entry => entry.enabled).length} 段启用
              </span>
              <button onClick={addEntry} className="flex items-center gap-1 text-xs text-primary-400 hover:text-primary-300">
                <Plus size={13} /> 添加
              </button>
            </div>

            <div className="space-y-1.5 max-h-[50vh] overflow-y-auto">
              {entries.map(entry => (
                <div
                  key={entry.id}
                  className={clsx(
                    'rounded-xl border transition-colors',
                    entry.enabled ? 'border-surface-border bg-surface' : 'border-surface-border/50 bg-dark-200 opacity-60'
                  )}
                >
                  <div
                    className="flex items-center gap-2 px-3 py-2 cursor-pointer"
                    onClick={() => setExpandedEntry(expandedEntry === entry.id ? null : entry.id)}
                  >
                    <div className="flex flex-col -my-1">
                      <button onClick={event => { event.stopPropagation(); moveEntry(entry.id, -1) }} className="text-gray-600 hover:text-gray-300 p-0.5">
                        <ChevronUp size={12} />
                      </button>
                      <button onClick={event => { event.stopPropagation(); moveEntry(entry.id, 1) }} className="text-gray-600 hover:text-gray-300 p-0.5">
                        <ChevronDown size={12} />
                      </button>
                    </div>

                    <span className={clsx('text-[10px] px-1.5 py-0.5 rounded border font-medium', ROLE_OPTIONS.find(role => role.value === entry.role)?.color || ROLE_OPTIONS[0].color)}>
                      {ROLE_OPTIONS.find(role => role.value === entry.role)?.label || '系统'}
                    </span>

                    <span className="text-sm font-medium flex-1 truncate">{entry.name}</span>

                    <button onClick={event => { event.stopPropagation(); toggleEntry(entry.id) }}>
                      {entry.enabled ? <ToggleRight size={18} className="text-primary-400" /> : <ToggleLeft size={18} className="text-gray-600" />}
                    </button>

                    <button onClick={event => { event.stopPropagation(); removeEntry(entry.id) }} className="text-gray-600 hover:text-red-400 transition-colors p-1">
                      <Trash2 size={13} />
                    </button>
                  </div>

                  {expandedEntry === entry.id && (
                    <div className="px-3 pb-3 pt-1 space-y-3 border-t border-surface-border/50">
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label className="block text-[10px] text-gray-500 mb-1">名称</label>
                          <input className="w-full input-base text-xs py-2" value={entry.name} onChange={event => updateEntry(entry.id, 'name', event.target.value)} />
                        </div>
                        <div>
                          <label className="block text-[10px] text-gray-500 mb-1">角色</label>
                          <select
                            className="w-full input-base text-xs py-2 bg-surface appearance-none"
                            value={entry.role}
                            onChange={event => updateEntry(entry.id, 'role', event.target.value)}
                          >
                            {ROLE_OPTIONS.map(role => (
                              <option key={role.value} value={role.value}>{role.label}</option>
                            ))}
                          </select>
                        </div>
                      </div>

                      <div>
                        <label className="block text-[10px] text-gray-500 mb-1">内容</label>
                        <textarea
                          className="w-full input-base text-xs resize-none py-2"
                          rows={5}
                          value={entry.content}
                          onChange={event => updateEntry(entry.id, 'content', event.target.value)}
                          placeholder="支持变量：{{char}} {{description}} {{personality}} {{scenario}} {{user}}"
                        />
                      </div>

                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label className="block text-[10px] text-gray-500 mb-1">注入方式</label>
                          <select
                            className="w-full input-base text-xs py-2 bg-surface appearance-none"
                            value={entry.injection_position}
                            onChange={event => updateEntry(entry.id, 'injection_position', Number(event.target.value))}
                          >
                            <option value={0}>相对末尾</option>
                            <option value={1}>绝对位置</option>
                          </select>
                        </div>
                        <div>
                          <label className="block text-[10px] text-gray-500 mb-1">注入深度</label>
                          <input
                            type="number"
                            min={0}
                            className="w-full input-base text-xs py-2"
                            value={entry.injection_depth}
                            onChange={event => updateEntry(entry.id, 'injection_depth', Number(event.target.value || 0))}
                          />
                        </div>
                      </div>

                      <label className="flex items-center gap-2 text-xs text-gray-300">
                        <input
                          type="checkbox"
                          checked={entry.system_prompt !== false}
                          onChange={event => updateEntry(entry.id, 'system_prompt', event.target.checked)}
                        />
                        作为 system prompt 段落处理
                      </label>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-3 gap-2">
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">Temperature</label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="2"
                className="w-full input-base"
                value={form.temperature}
                onChange={event => setForm(current => ({ ...current, temperature: Number(event.target.value || 0) }))}
              />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">Max Tokens</label>
              <input
                type="number"
                min="1"
                className="w-full input-base"
                value={form.max_tokens}
                onChange={event => setForm(current => ({ ...current, max_tokens: Number(event.target.value || 1) }))}
              />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">Top P</label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="1"
                className="w-full input-base"
                value={form.top_p}
                onChange={event => setForm(current => ({ ...current, top_p: Number(event.target.value || 0) }))}
              />
            </div>
          </div>

          <label className="flex items-center gap-2 text-sm text-gray-300">
            <input
              type="checkbox"
              checked={!!form.is_default}
              onChange={event => setForm(current => ({ ...current, is_default: event.target.checked }))}
            />
            设为默认预设
          </label>

          <div className="flex gap-2 pt-2">
            <button onClick={() => setEditPreset(null)} className="flex-1 py-2.5 rounded-xl border border-surface-border text-gray-300 hover:bg-surface-hover transition-colors">
              取消
            </button>
            <button onClick={handleSave} className="flex-1 btn-primary py-2.5">
              保存预设
            </button>
          </div>
        </div>
      </Modal>
    </div>
  )
}