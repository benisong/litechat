import React, { useEffect, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { MessageSquare, Users, BookOpen, Settings, Layers } from 'lucide-react'
import { useSettingsStore } from '../store'
import clsx from 'clsx'

export default function Layout() {
  const location = useLocation()
  const navigate = useNavigate()
  const { settings } = useSettingsStore()

  // 服务模式下隐藏预设和设置
  const isServiceMode = settings.service_mode === 'service'

  const NAV_ITEMS = [
    { path: '/chats',      label: '聊天',   icon: MessageSquare },
    { path: '/characters', label: '角色',   icon: Users },
    { path: '/worldbooks', label: '世界书', icon: BookOpen },
    // 自用模式才显示预设
    ...(!isServiceMode ? [{ path: '/presets', label: '预设', icon: Layers }] : []),
    { path: '/settings',   label: '设置',   icon: Settings },
  ]

  // 聊天详情页隐藏底部导航
  const isChatDetail = /^\/chats\/.+/.test(location.pathname)

  return (
    <div className="flex flex-col h-dvh max-w-lg mx-auto relative">
      <main className={clsx('flex-1 overflow-hidden', !isChatDetail && 'pb-16')}>
        <Outlet />
      </main>

      {!isChatDetail && (
        <nav className="fixed bottom-0 left-1/2 -translate-x-1/2 w-full max-w-lg
                        glass border-t border-surface-border
                        pb-[calc(env(safe-area-inset-bottom,0px)+0.25rem)] z-50">
          <div className="flex items-center justify-around h-16">
            {NAV_ITEMS.map(({ path, label, icon: Icon }) => {
              const active = location.pathname === path ||
                (path !== '/chats' && location.pathname.startsWith(path))
              return (
                <button
                  key={path}
                  onClick={() => navigate(path)}
                  className={clsx(
                    'flex flex-col items-center gap-0.5 px-3 py-2 rounded-xl transition-all duration-150',
                    active ? 'text-primary-400' : 'text-gray-500 active:text-gray-300'
                  )}
                >
                  <Icon size={20} strokeWidth={active ? 2.5 : 1.8} />
                  <span className={clsx('text-[10px] font-medium',
                    active ? 'text-primary-400' : 'text-gray-500'
                  )}>{label}</span>
                </button>
              )
            })}
          </div>
        </nav>
      )}
    </div>
  )
}
