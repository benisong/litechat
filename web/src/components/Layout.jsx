import React, { useEffect, useMemo, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { MessageSquare, Users, BookOpen, Settings, Layers } from 'lucide-react'
import clsx from 'clsx'
import { useAuthStore, useSettingsStore } from '../store'
import Modal from './ui/Modal'

export default function Layout() {
  const location = useLocation()
  const navigate = useNavigate()
  const { settings } = useSettingsStore()
  const user = useAuthStore(s => s.user)
  const [showProfilePrompt, setShowProfilePrompt] = useState(false)

  const isServiceMode = settings.service_mode === 'service'
  const isChatDetail = /^\/chats\/.+/.test(location.pathname)

  const navItems = useMemo(() => {
    const items = [
      { path: '/chats', label: '聊天', icon: MessageSquare },
      { path: '/characters', label: '角色', icon: Users },
      { path: '/worldbooks', label: '世界书', icon: BookOpen },
    ]

    if (!isServiceMode) {
      items.push({ path: '/presets', label: '预设', icon: Layers })
    }

    items.push({ path: '/settings', label: '设置', icon: Settings })
    return items
  }, [isServiceMode])

  const needsProfileSetup = user?.role !== 'admin' &&
    String(user?.user_name || '').trim().toLowerCase() === 'user'

  useEffect(() => {
    if (needsProfileSetup) {
      setShowProfilePrompt(true)
    } else {
      setShowProfilePrompt(false)
    }
  }, [needsProfileSetup, user?.id])

  return (
    <div className="relative mx-auto flex h-dvh max-w-lg min-h-0 flex-col">
      <main className={clsx('min-h-0 flex-1 overflow-hidden', !isChatDetail && 'pb-16')}>
        <Outlet />
      </main>

      {!isChatDetail && (
        <nav
          className="fixed bottom-0 left-1/2 z-50 w-full max-w-lg -translate-x-1/2 border-t border-surface-border glass pb-[calc(env(safe-area-inset-bottom,0px)+0.25rem)]"
        >
          <div className="flex h-16 items-center justify-around">
            {navItems.map(({ path, label, icon: Icon }) => {
              const active = location.pathname === path ||
                (path !== '/chats' && location.pathname.startsWith(path))

              return (
                <button
                  key={path}
                  onClick={() => navigate(path)}
                  className={clsx(
                    'flex flex-col items-center gap-0.5 rounded-xl px-3 py-2 transition-all duration-150',
                    active ? 'text-primary-400' : 'text-gray-500 active:text-gray-300'
                  )}
                >
                  <Icon size={20} strokeWidth={active ? 2.5 : 1.8} />
                  <span
                    className={clsx(
                      'text-[10px] font-medium',
                      active ? 'text-primary-400' : 'text-gray-500'
                    )}
                  >
                    {label}
                  </span>
                </button>
              )
            })}
          </div>
        </nav>
      )}

      <Modal
        open={showProfilePrompt}
        onClose={() => setShowProfilePrompt(false)}
        title="完善用户信息"
      >
        <div className="space-y-4">
          <p className="text-sm leading-6 text-gray-300">
            当前聊天用户信息还是默认值
            {' '}
            <span className="font-medium text-primary-300">user</span>
            ，建议先修改后再继续聊天，这样角色会更准确地识别你。
          </p>
          <div className="flex gap-3">
            <button
              onClick={() => setShowProfilePrompt(false)}
              className="flex-1 rounded-xl border border-surface-border py-3 text-gray-300 transition-colors hover:bg-surface-hover"
            >
              稍后
            </button>
            <button
              onClick={() => {
                setShowProfilePrompt(false)
                navigate('/settings')
              }}
              className="flex-1 btn-primary py-3"
            >
              去修改
            </button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
