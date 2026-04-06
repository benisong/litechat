import React, { useEffect, useState, useRef } from 'react'
import { Eye, EyeOff, Moon, Sun, Save, Cpu, RefreshCw, Loader2, Search, Check, LogOut,
         Monitor, Server } from 'lucide-react'
import { useSettingsStore, useUIStore, useAuthStore } from '../store'
import { useNavigate } from 'react-router-dom'
import clsx from 'clsx'

// 预设端点
const PRESET_ENDPOINTS = [
  { label: 'OpenAI', value: 'https://api.openai.com/v1' },
  { label: 'DeepSeek', value: 'https://api.deepseek.com/v1' },
  { label: 'Groq', value: 'https://api.groq.com/openai/v1' },
]

export default function SettingsPage() {
  const { settings, fetchSettings, saveSettings, setTheme } = useSettingsStore()
  const { showToast } = useUIStore()
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()
  const isAdmin = user?.role === 'admin'
  const isServiceMode = settings.service_mode === 'service'

  const [form, setForm] = useState({ ...settings })
  const [showKey, setShowKey] = useState(false)
  const [saving, setSaving] = useState(false)
  const endpointInputRef = useRef(null)

  // 模型相关
  const [models, setModels] = useState([])
  const [loadingModels, setLoadingModels] = useState(false)
  const [modelSearch, setModelSearch] = useState('')
  const [modelError, setModelError] = useState('')

  const isPresetEndpoint = PRESET_ENDPOINTS.some(ep => ep.value === form.api_endpoint)

  useEffect(() => {
    fetchSettings().then(() => {
      setForm({ ...useSettingsStore.getState().settings })
    }).catch(() => {})
  }, [])

  const handleFetchModels = async () => {
    if (!form.api_endpoint) { showToast('请先填写 API 端点', 'error'); return }
    if (!form.api_key) { showToast('请先填写 API 密钥', 'error'); return }
    setLoadingModels(true)
    setModelError('')
    try {
      const params = new URLSearchParams()
      params.set('endpoint', form.api_endpoint)
      if (!form.api_key.startsWith('***')) params.set('key', form.api_key)
      const token = JSON.parse(localStorage.getItem('litechat-auth') || '{}')?.state?.token
      const res = await fetch(`/api/models?${params}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || '获取失败')
      const list = (data.models || []).map(m => m.id).sort()
      setModels(list)
      if (list.length === 0) setModelError('该端点没有返回可用模型')
      else showToast(`获取到 ${list.length} 个模型`, 'success')
    } catch (err) {
      setModelError(err.message)
      showToast(err.message, 'error')
    } finally { setLoadingModels(false) }
  }

  const filteredModels = models.filter(m =>
    m.toLowerCase().includes(modelSearch.toLowerCase())
  )

  const handleSave = async () => {
    setSaving(true)
    try {
      await saveSettings(form)
      if (form.theme) setTheme(form.theme)
      showToast('设置已保存', 'success')
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    } finally { setSaving(false) }
  }

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  // 普通用户在服务模式下：只显示外观 + 退出
  const showAPIConfig = isAdmin || !isServiceMode

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 pt-12 pb-4">
        <h1 className="text-2xl font-bold">设置</h1>
      </div>

      <div className="flex-1 overflow-y-auto px-4 space-y-6 pb-6">
        {/* Admin: 模式切换 */}
        {isAdmin && (
          <section>
            <h2 className="text-xs text-gray-400 font-medium uppercase tracking-wider mb-3 px-1">
              运行模式
            </h2>
            <div className="card p-4">
              <div className="flex gap-3">
                {[
                  { value: 'self', label: '自用模式', desc: '用户可见预设和API配置', icon: Monitor },
                  { value: 'service', label: '服务模式', desc: '预设和API配置仅管理员可见', icon: Server },
                ].map(({ value, label, desc, icon: Icon }) => (
                  <button key={value}
                    onClick={() => setForm(f => ({ ...f, service_mode: value }))}
                    className={clsx(
                      'flex-1 flex flex-col items-center gap-1.5 py-4 rounded-xl border transition-all',
                      form.service_mode === value
                        ? 'border-primary-500/50 bg-primary-500/10 text-primary-300'
                        : 'border-surface-border text-gray-400 hover:bg-surface-hover'
                    )}>
                    <Icon size={20} />
                    <span className="text-sm font-medium">{label}</span>
                    <span className="text-[10px] text-gray-500">{desc}</span>
                  </button>
                ))}
              </div>
            </div>
          </section>
        )}

        {/* API 配置（admin 始终可见；用户仅自用模式可见） */}
        {showAPIConfig && (
          <section>
            <h2 className="text-xs text-gray-400 font-medium uppercase tracking-wider mb-3 px-1">
              API 配置
            </h2>
            <div className="card p-4 space-y-4">
              {/* 端点 */}
              <div>
                <label className="block text-xs text-gray-400 mb-2">API 端点</label>
                <div className="flex gap-2 flex-wrap mb-3">
                  {PRESET_ENDPOINTS.map(ep => (
                    <button key={ep.label}
                      onClick={() => { setForm(f => ({ ...f, api_endpoint: ep.value })); setModels([]) }}
                      className={clsx('text-xs px-3 py-1.5 rounded-lg border transition-colors',
                        form.api_endpoint === ep.value
                          ? 'bg-primary-500/20 border-primary-500/40 text-primary-300'
                          : 'border-surface-border text-gray-500 hover:text-gray-300 hover:bg-surface-hover'
                      )}>{ep.label}</button>
                  ))}
                  <button
                    onClick={() => { setForm(f => ({ ...f, api_endpoint: '' })); setModels([]); setTimeout(() => endpointInputRef.current?.focus(), 50) }}
                    className={clsx('text-xs px-3 py-1.5 rounded-lg border transition-colors',
                      !isPresetEndpoint ? 'bg-primary-500/20 border-primary-500/40 text-primary-300'
                        : 'border-surface-border text-gray-500 hover:text-gray-300 hover:bg-surface-hover'
                    )}>自定义</button>
                </div>
                <input ref={endpointInputRef} className="w-full input-base text-sm"
                  value={form.api_endpoint || ''}
                  onChange={e => { setForm(f => ({ ...f, api_endpoint: e.target.value })); setModels([]) }}
                  placeholder="输入第三方 API 地址，例如 https://your-proxy.com/v1" />
                {!isPresetEndpoint && form.api_endpoint && (
                  <p className="text-xs text-primary-400 mt-1.5 px-1">使用自定义端点：{form.api_endpoint}</p>
                )}
              </div>

              {/* 密钥 */}
              <div>
                <label className="block text-xs text-gray-400 mb-1.5">API 密钥</label>
                <div className="relative">
                  <input type={showKey ? 'text' : 'password'} className="w-full input-base text-sm pr-12"
                    value={form.api_key || ''}
                    onChange={e => setForm(f => ({ ...f, api_key: e.target.value }))}
                    placeholder="sk-..." />
                  <button onClick={() => setShowKey(v => !v)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300">
                    {showKey ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              {/* 模型 */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-xs text-gray-400">默认模型</label>
                  <button onClick={handleFetchModels} disabled={loadingModels}
                    className={clsx('flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg border transition-all',
                      'border-primary-500/40 text-primary-300 hover:bg-primary-500/10 active:scale-95',
                      loadingModels && 'opacity-60 cursor-not-allowed')}>
                    {loadingModels ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}
                    {loadingModels ? '获取中…' : '获取模型列表'}
                  </button>
                </div>
                {form.default_model && (
                  <div className="flex items-center gap-2 mb-3 px-3 py-2.5 bg-surface rounded-xl border border-surface-border">
                    <Check size={14} className="text-primary-400 flex-shrink-0" />
                    <span className="text-sm font-mono text-primary-300 truncate">{form.default_model}</span>
                  </div>
                )}
                {models.length > 0 && (
                  <div className="space-y-2">
                    <div className="relative">
                      <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
                      <input className="w-full input-base text-sm pl-9 py-2"
                        value={modelSearch} onChange={e => setModelSearch(e.target.value)}
                        placeholder={`搜索 ${models.length} 个模型…`} />
                    </div>
                    <div className="max-h-52 overflow-y-auto rounded-xl border border-surface-border bg-dark-200 divide-y divide-surface-border">
                      {filteredModels.length === 0 ? (
                        <p className="px-3 py-4 text-center text-xs text-gray-500">没有匹配的模型</p>
                      ) : filteredModels.map(m => (
                        <button key={m} onClick={() => setForm(f => ({ ...f, default_model: m }))}
                          className={clsx('w-full text-left px-3 py-2.5 text-sm font-mono transition-colors hover:bg-surface-hover',
                            form.default_model === m ? 'text-primary-300 bg-primary-500/10' : 'text-gray-300')}>
                          <div className="flex items-center gap-2">
                            {form.default_model === m && <Check size={13} className="text-primary-400 flex-shrink-0" />}
                            <span className="truncate">{m}</span>
                          </div>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
                {modelError && <p className="text-xs text-red-400 mt-2 px-1">{modelError}</p>}
                {models.length === 0 && (
                  <input className="w-full input-base text-sm mt-2"
                    value={form.default_model || ''}
                    onChange={e => setForm(f => ({ ...f, default_model: e.target.value }))}
                    placeholder="手动输入模型名称，或点击上方按钮获取列表" />
                )}
              </div>
            </div>
          </section>
        )}

        {/* 用户信息（admin 始终可见；用户仅自用模式可见） */}
        {showAPIConfig && (
          <section>
            <h2 className="text-xs text-gray-400 font-medium uppercase tracking-wider mb-3 px-1">
              用户信息
            </h2>
            <div className="card p-4 space-y-4">
              {/* 默认用户名称 */}
              <div>
                <label className="block text-xs text-gray-400 mb-1.5">默认用户名称</label>
                <input className="w-full input-base text-sm"
                  value={form.default_user_name || ''}
                  onChange={e => setForm(f => ({ ...f, default_user_name: e.target.value }))}
                  placeholder="输入用户名称" />
              </div>
              {/* 默认用户详情 */}
              <div>
                <label className="block text-xs text-gray-400 mb-1.5">默认用户详情</label>
                <textarea className="w-full input-base resize-none text-sm" rows={3}
                  value={form.default_user_detail || ''}
                  onChange={e => setForm(f => ({ ...f, default_user_detail: e.target.value }))}
                  placeholder="用户的背景设定、性格特征等" />
              </div>
            </div>
          </section>
        )}

        {/* 外观 */}
        <section>
          <h2 className="text-xs text-gray-400 font-medium uppercase tracking-wider mb-3 px-1">外观</h2>
          <div className="card p-4">
            <label className="block text-xs text-gray-400 mb-3">主题</label>
            <div className="flex gap-3">
              {[
                { value: 'dark', label: '深色', icon: Moon },
                { value: 'light', label: '浅色', icon: Sun },
              ].map(({ value, label, icon: Icon }) => (
                <button key={value} onClick={() => { setForm(f => ({ ...f, theme: value })); setTheme(value) }}
                  className={clsx('flex-1 flex items-center justify-center gap-2 py-3 rounded-xl border transition-all duration-150',
                    form.theme === value
                      ? 'border-primary-500/50 bg-primary-500/10 text-primary-300'
                      : 'border-surface-border text-gray-400 hover:bg-surface-hover')}>
                  <Icon size={16} />
                  <span className="text-sm font-medium">{label}</span>
                </button>
              ))}
            </div>
          </div>
        </section>

        {/* 关于 */}
        <section>
          <h2 className="text-xs text-gray-400 font-medium uppercase tracking-wider mb-3 px-1">关于</h2>
          <div className="card p-4 space-y-3">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-primary-500 to-purple-600 flex items-center justify-center">
                <Cpu size={20} className="text-white" />
              </div>
              <div>
                <p className="font-semibold">LiteChat</p>
                <p className="text-xs text-gray-500">轻量级 AI 角色聊天应用 v0.1.0</p>
              </div>
            </div>
          </div>
        </section>

        {/* 保存按钮 */}
        <button onClick={handleSave} disabled={saving}
          className="w-full btn-primary py-3.5 flex items-center justify-center gap-2 font-medium">
          <Save size={18} />
          {saving ? '保存中…' : '保存设置'}
        </button>

        {/* 退出登录（仅普通用户显示，admin 在自己界面有退出） */}
        {!isAdmin && (
          <button onClick={handleLogout}
            className="w-full py-3.5 rounded-xl border border-red-500/30 text-red-400
                       hover:bg-red-500/10 transition-colors flex items-center justify-center gap-2">
            <LogOut size={18} />
            退出登录
          </button>
        )}

        <div className="h-4" />
      </div>
    </div>
  )
}
