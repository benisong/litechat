import React, { useEffect, useMemo, useRef, useState } from 'react'
import {
  Eye,
  EyeOff,
  Moon,
  Sun,
  Save,
  Cpu,
  RefreshCw,
  Loader2,
  Search,
  Check,
  LogOut,
  Monitor,
  Server,
} from 'lucide-react'
import { useSettingsStore, useUIStore, useAuthStore } from '../store'
import { useNavigate } from 'react-router-dom'
import clsx from 'clsx'

const PRESET_ENDPOINTS = [
  { label: 'OpenAI', value: 'https://api.openai.com/v1' },
  { label: 'DeepSeek', value: 'https://api.deepseek.com/v1' },
  { label: 'Groq', value: 'https://api.groq.com/openai/v1' },
]

export default function SettingsPage() {
  const { settings, fetchSettings, saveSettings, setTheme } = useSettingsStore()
  const { showToast } = useUIStore()
  const { user, logout, updateProfile } = useAuthStore()
  const navigate = useNavigate()

  const isAdmin = user?.role === 'admin'
  const isServiceMode = settings.service_mode === 'service'
  const showAPIConfig = isAdmin || !isServiceMode
  const canSaveGlobalSettings = isAdmin || !isServiceMode

  const [form, setForm] = useState({ ...settings })
  const [showKey, setShowKey] = useState(false)
  const [saving, setSaving] = useState(false)
  const [savingProfile, setSavingProfile] = useState(false)
  const [models, setModels] = useState([])
  const [loadingModels, setLoadingModels] = useState(false)
  const [modelSearch, setModelSearch] = useState('')
  const [modelError, setModelError] = useState('')
  const [profileForm, setProfileForm] = useState({
    user_name: user?.user_name || 'user',
    user_detail: user?.user_detail || '',
  })
  const endpointInputRef = useRef(null)

  const isPresetEndpoint = PRESET_ENDPOINTS.some(ep => ep.value === form.api_endpoint)
  const filteredModels = useMemo(
    () => models.filter(m => m.toLowerCase().includes(modelSearch.toLowerCase())),
    [models, modelSearch]
  )
  const characterCardModelValue = form.use_default_model_for_character_card
    ? (form.default_model || '')
    : (form.character_card_model || form.default_model || '')

  useEffect(() => {
    fetchSettings()
      .then(() => setForm({ ...useSettingsStore.getState().settings }))
      .catch(() => {})
  }, [fetchSettings])

  useEffect(() => {
    setProfileForm({
      user_name: user?.user_name || 'user',
      user_detail: user?.user_detail || '',
    })
  }, [user?.id, user?.user_name, user?.user_detail])

  const handleFetchModels = async () => {
    if (!form.api_endpoint) {
      showToast('请先填写 API 端点', 'error')
      return
    }
    if (!form.api_key) {
      showToast('请先填写 API 密钥', 'error')
      return
    }

    setLoadingModels(true)
    setModelError('')
    try {
      const params = new URLSearchParams()
      params.set('endpoint', form.api_endpoint)
      if (!String(form.api_key || '').startsWith('***')) {
        params.set('key', form.api_key)
      }
      const token = JSON.parse(localStorage.getItem('litechat-auth') || '{}')?.state?.token
      const res = await fetch(`/api/models?${params.toString()}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || '获取失败')

      const list = (data.models || []).map(m => m.id).sort()
      setModels(list)
      if (list.length === 0) {
        setModelError('该端点没有返回可用模型')
      } else {
        showToast(`获取到 ${list.length} 个模型`, 'success')
      }
    } catch (err) {
      setModelError(err.message || '获取模型列表失败')
      showToast(err.message || '获取模型列表失败', 'error')
    } finally {
      setLoadingModels(false)
    }
  }

  useEffect(() => {
    if (!isAdmin) return
    if (form.use_default_model_for_character_card) return
    if (models.length > 0 || loadingModels) return
    handleFetchModels()
  }, [isAdmin, form.use_default_model_for_character_card])

  const handleSave = async () => {
    setSaving(true)
    try {
      await saveSettings(form)
      if (form.theme) setTheme(form.theme)
      showToast('设置已保存', 'success')
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleSaveProfile = async () => {
    if (isAdmin) return
    const userName = String(profileForm.user_name || '').trim()
    if (!userName) {
      showToast('用户名称不能为空', 'error')
      return
    }

    setSavingProfile(true)
    try {
      await updateProfile(userName, profileForm.user_detail || '')
      showToast('用户信息已保存', 'success')
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    } finally {
      setSavingProfile(false)
    }
  }

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="flex h-full flex-col">
      <div className="px-4 pb-4 pt-12">
        <h1 className="text-2xl font-bold">设置</h1>
      </div>

      <div className="flex-1 space-y-6 overflow-y-auto px-4 pb-6">
        {isAdmin && (
          <section>
            <h2 className="mb-3 px-1 text-xs font-medium uppercase tracking-wider text-gray-400">
              运行模式
            </h2>
            <div className="card p-4">
              <div className="flex gap-3">
                {[
                  { value: 'self', label: '自用模式', desc: '用户可见预设和 API 配置', icon: Monitor },
                  { value: 'service', label: '服务模式', desc: '预设和 API 配置仅管理员可见', icon: Server },
                ].map(({ value, label, desc, icon: Icon }) => (
                  <button
                    key={value}
                    onClick={() => setForm(f => ({ ...f, service_mode: value }))}
                    className={clsx(
                      'flex flex-1 flex-col items-center gap-1.5 rounded-xl border py-4 transition-all',
                      form.service_mode === value
                        ? 'border-primary-500/50 bg-primary-500/10 text-primary-300'
                        : 'border-surface-border text-gray-400 hover:bg-surface-hover'
                    )}
                  >
                    <Icon size={20} />
                    <span className="text-sm font-medium">{label}</span>
                    <span className="text-[10px] text-gray-500">{desc}</span>
                  </button>
                ))}
              </div>
            </div>
          </section>
        )}

        {showAPIConfig && (
          <section>
            <h2 className="mb-3 px-1 text-xs font-medium uppercase tracking-wider text-gray-400">
              API 配置
            </h2>
            <div className="card space-y-4 p-4">
              <div>
                <label className="mb-2 block text-xs text-gray-400">API 端点</label>
                <div className="mb-3 flex flex-wrap gap-2">
                  {PRESET_ENDPOINTS.map(ep => (
                    <button
                      key={ep.label}
                      onClick={() => {
                        setForm(f => ({ ...f, api_endpoint: ep.value }))
                        setModels([])
                      }}
                      className={clsx(
                        'rounded-lg border px-3 py-1.5 text-xs transition-colors',
                        form.api_endpoint === ep.value
                          ? 'border-primary-500/40 bg-primary-500/20 text-primary-300'
                          : 'border-surface-border text-gray-500 hover:bg-surface-hover hover:text-gray-300'
                      )}
                    >
                      {ep.label}
                    </button>
                  ))}
                  <button
                    onClick={() => {
                      setForm(f => ({ ...f, api_endpoint: '' }))
                      setModels([])
                      setTimeout(() => endpointInputRef.current?.focus(), 50)
                    }}
                    className={clsx(
                      'rounded-lg border px-3 py-1.5 text-xs transition-colors',
                      !isPresetEndpoint
                        ? 'border-primary-500/40 bg-primary-500/20 text-primary-300'
                        : 'border-surface-border text-gray-500 hover:bg-surface-hover hover:text-gray-300'
                    )}
                  >
                    自定义
                  </button>
                </div>
                <input
                  ref={endpointInputRef}
                  className="input-base w-full text-sm"
                  value={form.api_endpoint || ''}
                  onChange={e => {
                    setForm(f => ({ ...f, api_endpoint: e.target.value }))
                    setModels([])
                  }}
                  placeholder="输入第三方 API 地址，例如 https://your-proxy.com/v1"
                />
                {!isPresetEndpoint && form.api_endpoint && (
                  <p className="mt-1.5 px-1 text-xs text-primary-400">
                    使用自定义端点：{form.api_endpoint}
                  </p>
                )}
              </div>

              <div>
                <label className="mb-1.5 block text-xs text-gray-400">API 密钥</label>
                <div className="relative">
                  <input
                    type={showKey ? 'text' : 'password'}
                    className="input-base w-full pr-12 text-sm"
                    value={form.api_key || ''}
                    onChange={e => setForm(f => ({ ...f, api_key: e.target.value }))}
                    placeholder="sk-..."
                  />
                  <button
                    onClick={() => setShowKey(v => !v)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
                  >
                    {showKey ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              <div className="space-y-4">
                <div>
                  <div className="mb-2 flex items-center justify-between">
                    <label className="text-xs text-gray-400">默认模型</label>
                    <button
                      onClick={handleFetchModels}
                      disabled={loadingModels}
                      className={clsx(
                        'flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs transition-all',
                        'border-primary-500/40 text-primary-300 hover:bg-primary-500/10 active:scale-95',
                        loadingModels && 'cursor-not-allowed opacity-60'
                      )}
                    >
                      {loadingModels ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}
                      {loadingModels ? '获取中...' : '获取模型列表'}
                    </button>
                  </div>

                  {form.default_model && (
                    <div className="mb-3 flex items-center gap-2 rounded-xl border border-surface-border bg-surface px-3 py-2.5">
                      <Check size={14} className="flex-shrink-0 text-primary-400" />
                      <span className="truncate font-mono text-sm text-primary-300">{form.default_model}</span>
                    </div>
                  )}

                  {models.length > 0 ? (
                    <div className="space-y-2">
                      <div className="relative">
                        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
                        <input
                          className="input-base w-full py-2 pl-9 text-sm"
                          value={modelSearch}
                          onChange={e => setModelSearch(e.target.value)}
                          placeholder={`搜索 ${models.length} 个模型`}
                        />
                      </div>
                      <div className="max-h-52 overflow-y-auto rounded-xl border border-surface-border bg-dark-200 divide-y divide-surface-border">
                        {filteredModels.length === 0 ? (
                          <p className="px-3 py-4 text-center text-xs text-gray-500">没有匹配的模型</p>
                        ) : filteredModels.map(m => (
                          <button
                            key={m}
                            onClick={() => setForm(f => ({ ...f, default_model: m }))}
                            className={clsx(
                              'w-full px-3 py-2.5 text-left font-mono text-sm transition-colors hover:bg-surface-hover',
                              form.default_model === m ? 'bg-primary-500/10 text-primary-300' : 'text-gray-300'
                            )}
                          >
                            <div className="flex items-center gap-2">
                              {form.default_model === m && <Check size={13} className="flex-shrink-0 text-primary-400" />}
                              <span className="truncate">{m}</span>
                            </div>
                          </button>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <input
                      className="input-base mt-2 w-full text-sm"
                      value={form.default_model || ''}
                      onChange={e => setForm(f => ({ ...f, default_model: e.target.value }))}
                      placeholder="手动输入模型名称，或点击上方按钮获取列表"
                    />
                  )}
                </div>

                {isAdmin && (
                  <div className="space-y-3 rounded-xl border border-surface-border bg-surface/40 p-4">
                    <label className="flex cursor-pointer items-start gap-3">
                      <input
                        type="checkbox"
                        className="mt-1"
                        checked={form.use_default_model_for_character_card !== false}
                        onChange={e => {
                          const checked = e.target.checked
                          setForm(f => ({
                            ...f,
                            use_default_model_for_character_card: checked,
                            character_card_model: f.character_card_model || f.default_model || '',
                          }))
                        }}
                      />
                      <div>
                        <p className="text-sm text-gray-200">使用当前模型生成角色卡</p>
                        <p className="mt-1 text-xs text-gray-500">
                          {form.use_default_model_for_character_card !== false
                            ? '角色卡生成将跟随当前默认模型'
                            : '角色卡生成将使用独立模型，不影响聊天默认模型'}
                        </p>
                      </div>
                    </label>

                    <div>
                      <label className="mb-1.5 block text-xs text-gray-400">角色卡生成模型</label>
                      {models.length > 0 ? (
                        <select
                          className="input-base w-full appearance-none bg-surface text-sm disabled:opacity-60"
                          disabled={form.use_default_model_for_character_card !== false}
                          value={characterCardModelValue}
                          onChange={e => setForm(f => ({ ...f, character_card_model: e.target.value }))}
                        >
                          {models.map(m => (
                            <option key={m} value={m}>{m}</option>
                          ))}
                        </select>
                      ) : (
                        <input
                          className="input-base w-full text-sm disabled:opacity-60"
                          disabled={form.use_default_model_for_character_card !== false}
                          value={characterCardModelValue}
                          onChange={e => setForm(f => ({ ...f, character_card_model: e.target.value }))}
                          placeholder="先获取模型列表，或手动输入角色卡生成模型"
                        />
                      )}
                    </div>
                  </div>
                )}

                {modelError && <p className="px-1 text-xs text-red-400">{modelError}</p>}
              </div>
            </div>
          </section>
        )}

        {!isAdmin && (
          <section>
            <h2 className="mb-3 px-1 text-xs font-medium uppercase tracking-wider text-gray-400">
              用户信息
            </h2>
            <div className="card space-y-4 p-4">
              <div>
                <label className="mb-1.5 block text-xs text-gray-400">用户名称</label>
                <input
                  className="input-base w-full text-sm"
                  value={profileForm.user_name}
                  onChange={e => setProfileForm(f => ({ ...f, user_name: e.target.value }))}
                  placeholder="输入用户名称"
                />
                <p className="mt-1.5 px-1 text-xs text-gray-500">
                  该名称会用于聊天里的 <code>{'{{user}}'}</code> 变量和默认用户信息。
                </p>
              </div>
              <div>
                <label className="mb-1.5 block text-xs text-gray-400">用户详情</label>
                <textarea
                  className="input-base w-full resize-none text-sm"
                  rows={3}
                  value={profileForm.user_detail}
                  onChange={e => setProfileForm(f => ({ ...f, user_detail: e.target.value }))}
                  placeholder="你的背景设定、性格特征等"
                />
              </div>
              <button
                onClick={handleSaveProfile}
                disabled={savingProfile}
                className="w-full rounded-xl border border-primary-500/40 py-2.5 text-sm font-medium text-primary-300 transition-colors hover:bg-primary-500/10 disabled:opacity-60"
              >
                {savingProfile ? '保存中...' : '保存用户信息'}
              </button>
            </div>
          </section>
        )}

        <section>
          <h2 className="mb-3 px-1 text-xs font-medium uppercase tracking-wider text-gray-400">外观</h2>
          <div className="card p-4">
            <label className="mb-3 block text-xs text-gray-400">主题</label>
            <div className="flex gap-3">
              {[
                { value: 'dark', label: '深色', icon: Moon },
                { value: 'light', label: '浅色', icon: Sun },
              ].map(({ value, label, icon: Icon }) => (
                <button
                  key={value}
                  onClick={() => {
                    setForm(f => ({ ...f, theme: value }))
                    setTheme(value)
                  }}
                  className={clsx(
                    'flex flex-1 items-center justify-center gap-2 rounded-xl border py-3 transition-all duration-150',
                    form.theme === value
                      ? 'border-primary-500/50 bg-primary-500/10 text-primary-300'
                      : 'border-surface-border text-gray-400 hover:bg-surface-hover'
                  )}
                >
                  <Icon size={16} />
                  <span className="text-sm font-medium">{label}</span>
                </button>
              ))}
            </div>
          </div>
        </section>

        <section>
          <h2 className="mb-3 px-1 text-xs font-medium uppercase tracking-wider text-gray-400">关于</h2>
          <div className="card space-y-3 p-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 to-purple-600">
                <Cpu size={20} className="text-white" />
              </div>
              <div>
                <p className="font-semibold">LiteChat</p>
                <p className="text-xs text-gray-500">轻量级 AI 角色聊天应用 v0.1.0</p>
              </div>
            </div>
          </div>
        </section>

        {canSaveGlobalSettings && (
          <button
            onClick={handleSave}
            disabled={saving}
            className="btn-primary flex w-full items-center justify-center gap-2 py-3.5 font-medium"
          >
            <Save size={18} />
            {saving ? '保存中...' : '保存设置'}
          </button>
        )}

        {!isAdmin && (
          <button
            onClick={handleLogout}
            className="flex w-full items-center justify-center gap-2 rounded-xl border border-red-500/30 py-3.5 text-red-400 transition-colors hover:bg-red-500/10"
          >
            <LogOut size={18} />
            退出登录
          </button>
        )}

        <div className="h-4" />
      </div>
    </div>
  )
}
