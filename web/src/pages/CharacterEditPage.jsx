import React, { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { ChevronLeft, Save, Upload } from 'lucide-react'
import { useCharacterStore, useUIStore } from '../store'
import Avatar from '../components/ui/Avatar'

const FIELD_LABELS = {
  name: { label: '角色名称 *', placeholder: '例如：爱丽丝', type: 'input' },
  description: { label: '角色描述', placeholder: '简短描述角色的外貌、背景等', type: 'textarea', rows: 3 },
  personality: { label: '性格设定', placeholder: '角色的性格特点、行为模式', type: 'textarea', rows: 3 },
  scenario: { label: '场景设定', placeholder: '当前故事背景或场景', type: 'textarea', rows: 2 },
  first_msg: { label: '开场白', placeholder: '角色在对话开始时说的第一句话', type: 'textarea', rows: 3 },
  avatar_url: { label: '头像 URL', placeholder: 'https://...（可选）', type: 'input' },
  tags: { label: '标签', placeholder: '用逗号分隔，例如：女性,现代,温柔', type: 'input' },
}

const FORM_FIELDS = [
  'name',
  'description',
  'personality',
  'scenario',
  'first_msg',
  'avatar_url',
  'tags',
  'use_custom_user',
  'user_name',
  'user_detail',
]

const EMPTY_FORM = {
  name: '',
  description: '',
  personality: '',
  scenario: '',
  first_msg: '',
  avatar_url: '',
  tags: '',
  use_custom_user: false,
  user_name: '',
  user_detail: '',
}

function sanitizeFormData(data = {}) {
  const cleaned = { ...EMPTY_FORM }
  FORM_FIELDS.forEach(key => {
    cleaned[key] = data[key] ?? EMPTY_FORM[key]
  })
  cleaned.use_custom_user = !!data.use_custom_user
  return cleaned
}

export default function CharacterEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const { createCharacter, updateCharacter, fetchCharacter } = useCharacterStore()
  const { showToast } = useUIStore()
  const isNew = !id

  const draftFromTemplate = useMemo(
    () => (isNew ? location.state?.generatedDraft : null),
    [isNew, location.state]
  )

  const [form, setForm] = useState(() => sanitizeFormData(draftFromTemplate || EMPTY_FORM))
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!isNew) {
      fetchCharacter(id)
        .then(data => setForm(sanitizeFormData(data)))
        .catch(() => {
          showToast('加载失败', 'error')
          navigate('/characters')
        })
      return
    }

    if (draftFromTemplate) {
      setForm(sanitizeFormData(draftFromTemplate))
    }
  }, [id, isNew, draftFromTemplate])

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
      <div className="glass border-b border-surface-border px-4 flex items-center gap-3 pt-[env(safe-area-inset-top)] h-[calc(56px+env(safe-area-inset-top))]">
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
          {saving ? '保存中...' : '保存'}
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-6 space-y-5">
        {draftFromTemplate && (
          <div className="rounded-xl border border-primary-500/30 bg-primary-500/10 px-4 py-3">
            <p className="text-sm text-primary-200">AI 已生成角色卡草稿，你可以先检查和微调，再保存。</p>
          </div>
        )}

        <div className="flex justify-center">
          <div className="relative">
            <Avatar name={form.name} src={form.avatar_url} size="xl" />
            <div className="absolute -bottom-1 -right-1 w-7 h-7 bg-primary-600 rounded-xl flex items-center justify-center border-2 border-dark-400">
              <Upload size={12} />
            </div>
          </div>
        </div>

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

        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="block text-xs text-gray-400 font-medium">设置用户角色信息</label>
            <button
              onClick={() => setForm(f => ({ ...f, use_custom_user: !f.use_custom_user }))}
              className="flex-shrink-0"
            >
              {form.use_custom_user ? (
                <div className="w-10 h-5 rounded-full bg-primary-500 flex items-center justify-end px-0.5 transition-colors">
                  <div className="w-4 h-4 rounded-full bg-white" />
                </div>
              ) : (
                <div className="w-10 h-5 rounded-full bg-gray-600 flex items-center justify-start px-0.5 transition-colors">
                  <div className="w-4 h-4 rounded-full bg-white" />
                </div>
              )}
            </button>
          </div>

          {!form.use_custom_user ? (
            <p className="text-xs text-gray-500">未启用自定义用户信息时，将使用当前账户的用户资料。</p>
          ) : (
            <div className="space-y-3 mt-2">
              <div>
                <label className="block text-xs text-gray-400 mb-1.5 font-medium">用户名称</label>
                <input
                  type="text"
                  value={form.user_name || ''}
                  onChange={e => setForm(f => ({ ...f, user_name: e.target.value }))}
                  placeholder="输入用户名称"
                  className="w-full input-base"
                />
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1.5 font-medium">用户详情</label>
                <textarea
                  value={form.user_detail || ''}
                  onChange={e => setForm(f => ({ ...f, user_detail: e.target.value }))}
                  placeholder="用户的背景设定、性格特征等"
                  rows={3}
                  className="w-full input-base resize-none"
                />
              </div>
            </div>
          )}
        </div>

        <div className="bg-surface/50 rounded-xl p-4 border border-surface-border">
          <p className="text-xs text-gray-500 leading-relaxed">
            提示：在系统提示词模板中，可以使用 <code className="text-primary-400">{'{{char}}'}</code>、
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
