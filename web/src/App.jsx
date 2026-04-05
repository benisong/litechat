import React, { useEffect } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useSettingsStore } from './store'
import Layout from './components/Layout'
import ChatsPage from './pages/ChatsPage'
import ChatPage from './pages/ChatPage'
import CharactersPage from './pages/CharactersPage'
import CharacterEditPage from './pages/CharacterEditPage'
import PresetsPage from './pages/PresetsPage'
import WorldBooksPage from './pages/WorldBooksPage'
import SettingsPage from './pages/SettingsPage'
import Toast from './components/ui/Toast'

export default function App() {
  const { settings, fetchSettings, setTheme } = useSettingsStore()

  useEffect(() => {
    fetchSettings()
  }, [])

  useEffect(() => {
    // 应用主题
    document.documentElement.className = settings.theme || 'dark'
  }, [settings.theme])

  return (
    <>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/chats" replace />} />
          <Route path="chats" element={<ChatsPage />} />
          <Route path="chats/:chatId" element={<ChatPage />} />
          <Route path="characters" element={<CharactersPage />} />
          <Route path="characters/new" element={<CharacterEditPage />} />
          <Route path="characters/:id/edit" element={<CharacterEditPage />} />
          <Route path="presets" element={<PresetsPage />} />
          <Route path="worldbooks" element={<WorldBooksPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
      <Toast />
    </>
  )
}
