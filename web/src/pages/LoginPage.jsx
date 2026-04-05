import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Cpu, Eye, EyeOff, Loader2 } from 'lucide-react'
import { useAuthStore, useUIStore } from '../store'

export default function LoginPage() {
  const navigate = useNavigate()
  const { login } = useAuthStore()
  const { showToast } = useUIStore()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPwd, setShowPwd] = useState(false)
  const [loading, setLoading] = useState(false)

  const handleLogin = async (e) => {
    e.preventDefault()
    if (!username.trim() || !password) return
    setLoading(true)
    try {
      await login(username.trim(), password)
      navigate('/chats', { replace: true })
    } catch (err) {
      showToast(err.message || '登录失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="h-dvh flex flex-col items-center justify-center px-6 bg-dark-400">
      {/* Logo */}
      <div className="flex flex-col items-center mb-10">
        <div className="w-16 h-16 rounded-3xl bg-gradient-to-br from-primary-500 to-purple-600
                        flex items-center justify-center mb-4 shadow-xl shadow-primary-600/20">
          <Cpu size={32} className="text-white" />
        </div>
        <h1 className="text-2xl font-bold gradient-text">LiteChat</h1>
        <p className="text-xs text-gray-500 mt-1">轻量级 AI 角色聊天</p>
      </div>

      {/* 登录表单 */}
      <form onSubmit={handleLogin} className="w-full max-w-sm space-y-4">
        <div>
          <label className="block text-xs text-gray-400 mb-1.5">用户名</label>
          <input
            type="text"
            value={username}
            onChange={e => setUsername(e.target.value)}
            placeholder="输入用户名"
            className="w-full input-base"
            autoFocus
            autoComplete="username"
          />
        </div>

        <div>
          <label className="block text-xs text-gray-400 mb-1.5">密码</label>
          <div className="relative">
            <input
              type={showPwd ? 'text' : 'password'}
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="输入密码"
              className="w-full input-base pr-12"
              autoComplete="current-password"
            />
            <button type="button"
              onClick={() => setShowPwd(v => !v)}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300">
              {showPwd ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>
        </div>

        <button
          type="submit"
          disabled={loading || !username.trim() || !password}
          className="w-full btn-primary py-3.5 flex items-center justify-center gap-2 font-medium
                     disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {loading ? <Loader2 size={18} className="animate-spin" /> : null}
          {loading ? '登录中…' : '登录'}
        </button>
      </form>
    </div>
  )
}
