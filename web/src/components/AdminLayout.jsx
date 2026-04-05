import React from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Users, Settings, LogOut, Cpu, Layers } from 'lucide-react'
import { useAuthStore } from '../store'
import clsx from 'clsx'

const NAV_ITEMS = [
  { path: '/admin/users',    label: '用户管理', icon: Users },
  { path: '/admin/presets',  label: '预设管理', icon: Layers },
  { path: '/admin/settings', label: '系统设置', icon: Settings },
]

export default function AdminLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const { user, logout } = useAuthStore()

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="flex flex-col h-dvh max-w-lg mx-auto">
      {/* 顶部标题栏 */}
      <div className="glass border-b border-surface-border px-4 flex items-center gap-3
                      pt-[env(safe-area-inset-top)] h-[calc(56px+env(safe-area-inset-top))]">
        <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-amber-500 to-orange-600
                        flex items-center justify-center">
          <Cpu size={16} className="text-white" />
        </div>
        <div className="flex-1">
          <h1 className="font-semibold text-sm">LiteChat 管理</h1>
          <p className="text-[10px] text-gray-500">{user?.username}</p>
        </div>
        <button onClick={handleLogout} className="btn-ghost p-2 text-gray-400 hover:text-red-400">
          <LogOut size={18} />
        </button>
      </div>

      {/* 内容区 */}
      <main className="flex-1 overflow-hidden pb-16">
        <Outlet />
      </main>

      {/* 底部导航 */}
      <nav className="fixed bottom-0 left-1/2 -translate-x-1/2 w-full max-w-lg
                      glass border-t border-surface-border
                      pb-[env(safe-area-inset-bottom)] z-50">
        <div className="flex items-center justify-around h-16">
          {NAV_ITEMS.map(({ path, label, icon: Icon }) => {
            const active = location.pathname === path
            return (
              <button
                key={path}
                onClick={() => navigate(path)}
                className={clsx(
                  'flex flex-col items-center gap-0.5 px-6 py-2 rounded-xl transition-all duration-150',
                  active ? 'text-amber-400' : 'text-gray-500 active:text-gray-300'
                )}
              >
                <Icon size={22} strokeWidth={active ? 2.5 : 1.8} />
                <span className={clsx('text-[10px] font-medium',
                  active ? 'text-amber-400' : 'text-gray-500'
                )}>
                  {label}
                </span>
              </button>
            )
          })}
        </div>
      </nav>
    </div>
  )
}
