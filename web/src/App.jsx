import React, { useEffect } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useSettingsStore, useAuthStore } from './store'
import Layout from './components/Layout'
import AdminLayout from './components/AdminLayout'
import LoginPage from './pages/LoginPage'
import ChatsPage from './pages/ChatsPage'
import ChatPage from './pages/ChatPage'
import CharactersPage from './pages/CharactersPage'
import CharacterEditPage from './pages/CharacterEditPage'
import PresetsPage from './pages/PresetsPage'
import WorldBooksPage from './pages/WorldBooksPage'
import SettingsPage from './pages/SettingsPage'
import UsersPage from './pages/UsersPage'
import Toast from './components/ui/Toast'

export default function App() {
  const { settings, fetchSettings } = useSettingsStore()
  const { token, user } = useAuthStore()
  const isLoggedIn = !!token
  const isAdmin = user?.role === 'admin'
  const isServiceMode = settings.service_mode === 'service'

  useEffect(() => {
    if (isLoggedIn) fetchSettings().catch(() => {})
  }, [isLoggedIn])

  useEffect(() => {
    document.documentElement.className = settings.theme || 'dark'
  }, [settings.theme])

  // 未登录
  if (!isLoggedIn) {
    return (
      <>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
        <Toast />
      </>
    )
  }

  // Admin：管理面板
  if (isAdmin) {
    return (
      <>
        <Routes>
          <Route path="/login" element={<Navigate to="/admin/users" replace />} />
          <Route path="/admin" element={<AdminLayout />}>
            <Route index element={<Navigate to="/admin/users" replace />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="presets" element={<PresetsPage />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/admin/users" replace />} />
        </Routes>
        <Toast />
      </>
    )
  }

  // 普通用户：聊天界面
  return (
    <>
      <Routes>
        <Route path="/login" element={<Navigate to="/chats" replace />} />
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/chats" replace />} />
          <Route path="chats" element={<ChatsPage />} />
          <Route path="chats/:chatId" element={<ChatPage />} />
          <Route path="characters" element={<CharactersPage />} />
          <Route path="characters/new" element={<CharacterEditPage />} />
          <Route path="characters/:id/edit" element={<CharacterEditPage />} />
          {/* 自用模式：用户可见预设 */}
          {!isServiceMode && <Route path="presets" element={<PresetsPage />} />}
          <Route path="worldbooks" element={<WorldBooksPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/chats" replace />} />
      </Routes>
      <Toast />
    </>
  )
}
