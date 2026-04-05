import React from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { MessageSquare, Users, BookOpen, Settings, Layers } from 'lucide-react'
import clsx from 'clsx'

const NAV_ITEMS = [
  { path: '/chats',      label: '聊天',   icon: MessageSquare },
  { path: '/characters', label: '角色',   icon: Users },
  { path: '/worldbooks', label: '世界书', icon: BookOpen },
  { path: '/presets',    label: '预设',   icon: Layers },
  { path: '/settings',   label: '设置',   icon: Settings },
]

export default function Layout() {
  const location = useLocation()
  const navigate = useNavigate()

  // 聊天详情页隐藏底部导航
  const isChatDetail = /^\/chats\/.+/.test(location.pathname)

  return (
    <div className="flex flex-col h-dvh max-w-lg mx-auto relative">
      {/* 主内容区 */}
      <main className={clsx(
        'flex-1 overflow-hidden',
        !isChatDetail && 'pb-16' // 为底部导航留空间
      )}>
        <Outlet />
      </main>

      {/* 底部导航栏 */}
      {!isChatDetail && (
        <nav className="fixed bottom-0 left-1/2 -translate-x-1/2 w-full max-w-lg
                        glass border-t border-surface-border
                        pb-[env(safe-area-inset-bottom)]
                        z-50">
          <div className="flex items-center justify-around h-16">
            {NAV_ITEMS.map(({ path, label, icon: Icon }) => {
              const active = location.pathname === path ||
                (path !== '/chats' && location.pathname.startsWith(path))
              return (
                <button
                  key={path}
                  onClick={() => navigate(path)}
                  className={clsx(
                    'flex flex-col items-center gap-0.5 px-4 py-2 rounded-xl transition-all duration-150',
                    active
                      ? 'text-primary-400'
                      : 'text-gray-500 active:text-gray-300'
                  )}
                >
                  <Icon size={22} strokeWidth={active ? 2.5 : 1.8} />
                  <span className={clsx(
                    'text-[10px] font-medium',
                    active ? 'text-primary-400' : 'text-gray-500'
                  )}>
                    {label}
                  </span>
                  {/* 活跃指示点 */}
                  {active && (
                    <span className="absolute bottom-1 w-1 h-1 bg-primary-400 rounded-full" />
                  )}
                </button>
              )
            })}
          </div>
        </nav>
      )}
    </div>
  )
}
