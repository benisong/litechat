import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// ===== API 工具函数 =====
const BASE = '/api'

async function apiFetch(path, options = {}) {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
    body: options.body ? JSON.stringify(options.body) : undefined,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: '请求失败' }))
    throw new Error(err.error || '请求失败')
  }
  return res.json()
}

// ===== 角色卡 Store =====
export const useCharacterStore = create((set, get) => ({
  characters: [],
  loading: false,

  fetchCharacters: async () => {
    set({ loading: true })
    try {
      const data = await apiFetch('/characters')
      set({ characters: data || [] })
    } finally {
      set({ loading: false })
    }
  },

  createCharacter: async (char) => {
    const data = await apiFetch('/characters', { method: 'POST', body: char })
    set(s => ({ characters: [data, ...s.characters] }))
    return data
  },

  updateCharacter: async (id, char) => {
    const data = await apiFetch(`/characters/${id}`, { method: 'PUT', body: char })
    set(s => ({ characters: s.characters.map(c => c.id === id ? data : c) }))
    return data
  },

  deleteCharacter: async (id) => {
    await apiFetch(`/characters/${id}`, { method: 'DELETE' })
    set(s => ({ characters: s.characters.filter(c => c.id !== id) }))
  },
}))

// ===== 对话 Store =====
export const useChatStore = create((set, get) => ({
  chats: [],
  currentChat: null,
  messages: [],
  loading: false,
  streaming: false,
  streamContent: '',

  fetchChats: async (characterId) => {
    const url = characterId ? `/chats?character_id=${characterId}` : '/chats'
    const data = await apiFetch(url)
    set({ chats: data || [] })
  },

  createChat: async (characterId, title, presetId) => {
    const data = await apiFetch('/chats', {
      method: 'POST',
      body: { character_id: characterId, title: title || '新对话', preset_id: presetId || '' },
    })
    set(s => ({ chats: [data, ...s.chats] }))
    return data
  },

  setCurrentChat: (chat) => set({ currentChat: chat, messages: [] }),

  fetchMessages: async (chatId) => {
    set({ loading: true })
    try {
      const data = await apiFetch(`/chats/${chatId}/messages`)
      set({ messages: data || [] })
    } finally {
      set({ loading: false })
    }
  },

  // 发送消息（SSE 流式）
  sendMessage: async (chatId, content, presetId) => {
    const userMsg = {
      id: 'temp-' + Date.now(),
      chat_id: chatId,
      role: 'user',
      content,
      created_at: new Date().toISOString(),
    }
    set(s => ({ messages: [...s.messages, userMsg], streaming: true, streamContent: '' }))

    // 先添加一个空的 AI 消息占位
    const aiMsgPlaceholder = {
      id: 'temp-ai-' + Date.now(),
      chat_id: chatId,
      role: 'assistant',
      content: '',
      created_at: new Date().toISOString(),
      isStreaming: true,
    }
    set(s => ({ messages: [...s.messages, aiMsgPlaceholder] }))

    try {
      const res = await fetch(`${BASE}/chats/${chatId}/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content, preset_id: presetId || '' }),
      })

      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let fullContent = ''
      let buffer = '' // 行缓冲，处理跨 chunk 的 SSE 行
      let streamDone = false

      while (!streamDone) {
        const { done, value } = await reader.read()
        if (done) break

        // 追加到缓冲区（stream: true 确保多字节字符不被截断）
        buffer += decoder.decode(value, { stream: true })

        // 按双换行分割完整的 SSE 事件
        const parts = buffer.split('\n')
        // 最后一段可能不完整，保留在 buffer 中
        buffer = parts.pop() || ''

        for (const line of parts) {
          const trimmed = line.trim()
          if (!trimmed.startsWith('data:')) continue
          const data = trimmed.slice(5).trim()
          if (!data) continue

          try {
            const parsed = JSON.parse(data)
            if (parsed.done) { streamDone = true; break }
            if (parsed.error) throw new Error(parsed.error)
            if (parsed.token) {
              fullContent += parsed.token
              // 更新流式 AI 消息
              set(s => ({
                messages: s.messages.map(m =>
                  m.id === aiMsgPlaceholder.id
                    ? { ...m, content: fullContent }
                    : m
                ),
                streamContent: fullContent,
              }))
            }
          } catch (e) {
            // 仅在非 JSON 解析错误时抛出
            if (e.message && !e.message.includes('JSON')) throw e
          }
        }
      }

      // 流式结束，刷新消息列表
      const freshMessages = await apiFetch(`/chats/${chatId}/messages`)
      set({ messages: freshMessages || [], streaming: false, streamContent: '' })
    } catch (err) {
      set(s => ({
        messages: s.messages.filter(m => m.id !== aiMsgPlaceholder.id),
        streaming: false,
      }))
      throw err
    }
  },

  deleteChat: async (id) => {
    await apiFetch(`/chats/${id}`, { method: 'DELETE' })
    set(s => ({ chats: s.chats.filter(c => c.id !== id) }))
  },

  deleteMessage: async (id) => {
    await apiFetch(`/messages/${id}`, { method: 'DELETE' })
    set(s => ({ messages: s.messages.filter(m => m.id !== id) }))
  },
}))

// ===== 预设 Store =====
export const usePresetStore = create((set) => ({
  presets: [],

  fetchPresets: async () => {
    const data = await apiFetch('/presets')
    set({ presets: data || [] })
  },

  createPreset: async (preset) => {
    const data = await apiFetch('/presets', { method: 'POST', body: preset })
    set(s => ({ presets: [...s.presets, data] }))
    return data
  },

  updatePreset: async (id, preset) => {
    const data = await apiFetch(`/presets/${id}`, { method: 'PUT', body: preset })
    set(s => ({ presets: s.presets.map(p => p.id === id ? data : p) }))
    return data
  },

  deletePreset: async (id) => {
    await apiFetch(`/presets/${id}`, { method: 'DELETE' })
    set(s => ({ presets: s.presets.filter(p => p.id !== id) }))
  },
}))

// ===== 世界书 Store =====
export const useWorldBookStore = create((set) => ({
  worldBooks: [],
  currentBook: null,

  fetchWorldBooks: async () => {
    const data = await apiFetch('/worldbooks')
    set({ worldBooks: data || [] })
  },

  createWorldBook: async (wb) => {
    const data = await apiFetch('/worldbooks', { method: 'POST', body: wb })
    set(s => ({ worldBooks: [data, ...s.worldBooks] }))
    return data
  },

  fetchWorldBook: async (id) => {
    const data = await apiFetch(`/worldbooks/${id}`)
    set({ currentBook: data })
    return data
  },

  updateWorldBook: async (id, wb) => {
    const data = await apiFetch(`/worldbooks/${id}`, { method: 'PUT', body: wb })
    set(s => ({ worldBooks: s.worldBooks.map(b => b.id === id ? data : b) }))
    return data
  },

  deleteWorldBook: async (id) => {
    await apiFetch(`/worldbooks/${id}`, { method: 'DELETE' })
    set(s => ({ worldBooks: s.worldBooks.filter(b => b.id !== id) }))
  },

  createEntry: async (worldBookId, entry) => {
    const data = await apiFetch(`/worldbooks/${worldBookId}/entries`, { method: 'POST', body: entry })
    set(s => ({
      currentBook: s.currentBook ? {
        ...s.currentBook,
        entries: [...(s.currentBook.entries || []), data]
      } : null
    }))
    return data
  },

  updateEntry: async (entryId, entry) => {
    const data = await apiFetch(`/worldbooks/entries/${entryId}`, { method: 'PUT', body: entry })
    set(s => ({
      currentBook: s.currentBook ? {
        ...s.currentBook,
        entries: (s.currentBook.entries || []).map(e => e.id === entryId ? data : e)
      } : null
    }))
    return data
  },

  deleteEntry: async (entryId) => {
    await apiFetch(`/worldbooks/entries/${entryId}`, { method: 'DELETE' })
    set(s => ({
      currentBook: s.currentBook ? {
        ...s.currentBook,
        entries: (s.currentBook.entries || []).filter(e => e.id !== entryId)
      } : null
    }))
  },
}))

// ===== 设置 Store（持久化到 localStorage）=====
export const useSettingsStore = create(
  persist(
    (set, get) => ({
      settings: {
        api_endpoint: 'https://api.openai.com/v1',
        api_key: '',
        default_model: 'gpt-4o-mini',
        theme: 'dark',
      },
      loaded: false,

      fetchSettings: async () => {
        try {
          const data = await apiFetch('/settings')
          set({ settings: data, loaded: true })
        } catch {}
      },

      saveSettings: async (settings) => {
        await apiFetch('/settings', { method: 'PUT', body: settings })
        set({ settings: { ...get().settings, ...settings } })
      },

      setTheme: (theme) => {
        set(s => ({ settings: { ...s.settings, theme } }))
        document.documentElement.className = theme
      },
    }),
    {
      name: 'litechat-settings',
      partialize: (state) => ({ settings: state.settings }),
    }
  )
)

// ===== UI Store =====
export const useUIStore = create((set) => ({
  toast: null,
  showToast: (message, type = 'info') => {
    set({ toast: { message, type, id: Date.now() } })
    setTimeout(() => set({ toast: null }), 3000)
  },
}))
