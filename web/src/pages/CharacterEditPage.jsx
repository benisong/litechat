import React, { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ChevronLeft, Save, Upload, User } from 'lucide-react'
import { useCharacterStore, useUIStore } from '../store'
import Avatar from '../components/ui/Avatar'

const FIELD_LABELS = {
  name:        { label: '角色名称 *', placeholder: '例如：爱丽丝', type: 'input' },
  description: { label: '角色描述', placeholder: '简短描述角色的外貌、背景等', type: 'textarea', rows: 3 },
  personality: { label: '性格设定', placeholder: '角色的性格特点、行为模式', type: 'textarea', rows: 3 },
  scenario:    { label: '场景设定', placeholder: '当前故事背景或场景', type: 'textarea', rows: 2 },
  first_msg:   { label: '开场白', placeholder: '角色在对话开始时说的第一句话', type: 'textarea', rows: 3 },
  avatar_url:  { label: '头像 URL', placeholder: 'https://...（可选）', type: 'input' },
  tags:        { label: '标签', placeholder: '用逗号分隔，例如：女性,现代,温柔', type: 'input' },
}

export default function CharacterEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { createCharacter, updateCharacter } = useCharacterStore()
  const { showToast } = useUIStore()
  const isNew = !id

  const [form, setForm] = useState({
    name: '', description: '', personality: '',
    scenario: '', first_msg: '', avatar_url: '', tags: '',
  })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!isNew) {
      fetch(`/api/characters/${id}`)
        .then(r => r.json())
        .then(data => setForm(data))
        .catch(() => { showToast('加载失败', 'error'); navigate('/characters') })
    }
  }, [id])

  const handleSave = async () => {
    if (!form.name.trim()) {
      showToast('请填写角色名称', 'error')
      return
    }
    setSaving(true)
    try {
      if (isNew) {
        const char = await createCharacter(form)
        showToast('角色创建成功', 'success')
        navigate(`/characters/${char.id}/edit`, { replace: true })
      } else {
        await updateCharacter(id, form)
        showToast('保存成功', 'success')
      }
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col h-full">
      {/* 顶部导航 */}
      <div className="glass border-b border-surface-border px-4 flex items-center gap-3
                      pt-[env(safe-area-inset-top)] h-[calc(56px+env(safe-area-inset-top))]">
        <button onClick={() => navigate('/characters')} className="btn-ghost p-2 -ml-2">
          <ChevronLeft size={22} />
        </button>
        <h1 className="flex-1 font-semibold">{isNew ? '创建角色' : '编辑角色'}</h1>
        <button
          onClick={handleSave}
          disabled={saving}
          className="btn-primary flex items-center gap-2 py-2 px-4 text-sm"
        >
          <Save size={15} />
          {saving ? '保存中…' : '保存'}
        </button>
      </div>

      {/* 表单 */}
      <div className="flex-1 overflow-y-auto px-4 py-6 space-y-5">
        {/* 头像预览 */}
        <div className="flex justify-center">
          <div className="relative">
            <Avatar name={form.name} src={form.avatar_url} size="xl" />
            <div className="absolute -bottom-1 -right-1 w-7 h-7 bg-primary-600 rounded-xl
                            flex items-center justify-center border-2 border-dark-400">
              <Upload size={12} />
            </div>
          </div>
        </div>

        {/* 各字段 */}
        {Object.entries(FIELD_LABELS).map(([key, config]) => (
          <div key={key}>
            <label className="block text-xs text-gray-400 mb-1.5 font-medium">
              {config.label}
            </label>
            {config.type === 'textarea' ? (
              <textarea
                value={form[key]}
                onChange={e => setForm(f => ({ ...f, [key]: e.target.value }))}
                placeholder={config.placeholder}
                rows={config.rows}
                className="w-full input-base resize-none"
              />
            ) : (
              <input
                type="text"
                value={form[key]}
                onChange={e => setForm(f => ({ ...f, [key]: e.target.value }))}
                placeholder={config.placeholder}
                className="w-full input-base"
              />
            )}
          </div>
        ))}

        {/* 提示 */}
        <div className="bg-surface/50 rounded-xl p-4 border border-surface-border">
          <p className="text-xs text-gray-500 leading-relaxed">
            💡 提示：在系统提示词模板中，可以使用 <code className="text-primary-400">{'{{char}}'}</code>、
            <code className="text-primary-400">{'{{description}}'}</code>、
            <code className="text-primary-400">{'{{personality}}'}</code>、
            <code className="text-primary-400">{'{{scenario}}'}</code> 等变量引用角色信息。
          </p>
        </div>

        <div className="h-4" />
      </div>
    </div>
  )
}
