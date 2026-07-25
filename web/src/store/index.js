import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import { normalizeChatMessages } from '../utils/statusBar'

// ===== API 工具函数 =====
const BASE = '/api'
const AUTH_STORAGE_KEY = 'litechat-auth'
const authStorage = createJSONStorage(() => sessionStorage)
let messageFetchSerial = 0
let tempMessageSerial = 0
const sendingChatIds = new Set()

function createTempMessageId(prefix) {
  tempMessageSerial += 1
  const randomPart = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${tempMessageSerial}`
  return `${prefix}-${randomPart}`
}

function chatBusyError() {
  const error = new Error('上一条消息仍在发送，请稍候')
  error.code = 'CHAT_BUSY'
  error.canResend = true
  return error
}

const DEFAULT_SETTINGS = {
  api_endpoint: 'https://api.openai.com/v1',
  api_key: '',
  default_model: 'gpt-4o-mini',
  use_default_model_for_character_card: true,
  character_card_model: '',
  use_default_model_for_memory: true,
  memory_model: '',
  memory_prompt_suffix: '',
  memory_summary_char_limit: 3000,
  theme: 'dark',
  chat_font_size: '0.875rem',
  service_mode: 'self',
}

// 从 zustand persist 读取 token
function clearAuthPersistence() {
  try {
    sessionStorage.removeItem(AUTH_STORAGE_KEY)
  } catch {}
  try {
    localStorage.removeItem(AUTH_STORAGE_KEY)
  } catch {}
}

try {
  localStorage.removeItem(AUTH_STORAGE_KEY)
} catch {}

export function getToken() {
  try {
    const stored = sessionStorage.getItem(AUTH_STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored)
      return parsed?.state?.token || null
    }
  } catch {}
  return null
}

async function apiFetch(path, options = {}) {
  const token = getToken()
  const headers = { 'Content-Type': 'application/json', ...options.headers }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(BASE + path, {
    headers,
    ...options,
    body: options.body ? JSON.stringify(options.body) : undefined,
  })

  // 401 未认证 → 静默抛出错误，由调用方处理
  // 不做硬刷新，避免无限循环；路由守卫会在 token 清除后自动跳转登录页
  if (res.status === 401) {
    clearAuthPersistence()
    try {
      useAuthStore.setState({ user: null, token: null })
    } catch {}
    throw new Error('未授权，请重新登录')
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: '请求失败' }))
    throw new Error(err.error || '请求失败')
  }
  return res.json()
}

// ===== 认证 Store =====
export const useAuthStore = create(
  persist(
    (set, get) => ({
      user: null,
      token: null,

      login: async (username, password) => {
        const res = await fetch(`${BASE}/auth/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username, password }),
        })
        if (!res.ok) {
          const err = await res.json().catch(() => ({ error: '登录失败' }))
          throw new Error(err.error || '登录失败')
        }
        const data = await res.json()
        set({ user: data.user, token: data.token })
        return data
      },

      fetchMe: async () => {
        const data = await apiFetch('/auth/me')
        set({ user: data })
        return data
      },

      updateProfile: async (user_name, user_detail) => {
        const data = await apiFetch('/auth/me/profile', {
          method: 'PUT',
          body: { user_name, user_detail },
        })
        set(s => ({ user: { ...s.user, ...data } }))
        return data
      },

      logout: () => {
        clearAuthPersistence()
        set({ user: null, token: null })
      },

      isLoggedIn: () => !!get().token,
      isAdmin: () => get().user?.role === 'admin',
    }),
    {
      name: AUTH_STORAGE_KEY,
      storage: authStorage,
      partialize: (state) => ({ user: state.user, token: state.token }),
    }
  )
)

// ===== 用户管理 Store（管理员）=====
export const useUserStore = create((set) => ({
  users: [],
  fetchUsers: async () => {
    const data = await apiFetch('/auth/users')
    set({ users: data || [] })
  },
  createUser: async (username, password, role = 'user') => {
    const data = await apiFetch('/auth/users', {
      method: 'POST', body: { username, password, role },
    })
    set(s => ({ users: [...s.users, data] }))
    return data
  },
  deleteUser: async (id) => {
    await apiFetch(`/auth/users/${id}`, { method: 'DELETE' })
    set(s => ({ users: s.users.filter(u => u.id !== id) }))
  },
  changePassword: async (old_password, new_password) => {
    await apiFetch('/auth/password', {
      method: 'PUT', body: { old_password, new_password },
    })
  },
}))

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

  generateCharacterCard: async (choices) => {
    const data = await apiFetch('/characters/generate', {
      method: 'POST',
      body: choices,
    })
    return data?.draft || data
  },

  updateCharacter: async (id, char) => {
    const data = await apiFetch(`/characters/${id}`, { method: 'PUT', body: char })
    set(s => ({ characters: s.characters.map(c => c.id === id ? data : c) }))
    return data
  },

  fetchCharacter: async (id) => {
    return await apiFetch(`/characters/${id}`)
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
  activeChatId: null,
  messages: [],
  loading: false,
  streaming: false,
  streamingChatId: null,
  streamKind: null,
  streamBaseSeq: 0,
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

  setCurrentChat: (chat) => set({
    currentChat: chat,
    activeChatId: chat?.id || null,
    messages: [],
  }),

  fetchMessages: async (chatId, { background = false } = {}) => {
    if (background && get().activeChatId !== chatId) {
      return []
    }

    const fetchSerial = ++messageFetchSerial
    set(s => ({
      activeChatId: chatId,
      ...(s.activeChatId !== chatId ? { messages: [] } : {}),
      ...(!background ? { loading: true } : {}),
    }))

    try {
      const data = await apiFetch(`/chats/${chatId}/messages`)
      const normalizedMessages = normalizeChatMessages(data)
      set(s => {
        if (fetchSerial !== messageFetchSerial || s.activeChatId !== chatId) {
          return {}
        }

        // An older GET may finish after an optimistic send has started. Never
        // let that stale snapshot erase the just-sent message or its placeholder.
        if (s.streaming && s.streamingChatId === chatId) {
          if (background && s.streamKind === 'send') {
            const persistedUser = normalizedMessages.find(message => (
              message.seq > s.streamBaseSeq && message.role === 'user'
            ))
            if (persistedUser) {
              const persistedAssistant = normalizedMessages.find(message => (
                message.seq > persistedUser.seq && message.role === 'assistant'
              ))
              if (persistedAssistant) {
                return {
                  messages: normalizedMessages,
                  streaming: false,
                  streamingChatId: null,
                  streamKind: null,
                  streamBaseSeq: 0,
                  streamContent: '',
                }
              }

              const aiPlaceholder = s.messages.find(message => (
                message.chat_id === chatId
                && message.role === 'assistant'
                && message.isStreaming
              ))
              return {
                messages: aiPlaceholder
                  ? [...normalizedMessages, aiPlaceholder]
                  : normalizedMessages,
              }
            }
          }
          return {}
        }

        return { messages: normalizedMessages }
      })
      return normalizedMessages
    } finally {
      set(s => (
        fetchSerial === messageFetchSerial && s.activeChatId === chatId
          ? { loading: false }
          : {}
      ))
    }
  },

  // 发送消息（SSE 流式）
  sendMessage: async (chatId, content, presetId) => {
    if (sendingChatIds.has(chatId) || get().streaming) {
      throw chatBusyError()
    }
    sendingChatIds.add(chatId)

    const userMsg = {
      id: createTempMessageId('temp-user'),
      chat_id: chatId,
      role: 'user',
      content,
      created_at: new Date().toISOString(),
    }
    set(s => ({
      activeChatId: chatId,
      messages: [
        ...(s.activeChatId === chatId ? s.messages : []),
        userMsg,
      ],
      streaming: true,
      streamingChatId: chatId,
      streamKind: 'send',
      streamBaseSeq: s.activeChatId === chatId
        ? s.messages.reduce(
            (latest, message) => Math.max(latest, Number(message.seq) || 0),
            0
          )
        : 0,
      streamContent: '',
    }))

    const requestStartedAt = Date.now()

    // 先添加一个空的 AI 消息占位
    const aiMsgPlaceholder = {
      id: createTempMessageId('temp-ai'),
      chat_id: chatId,
      role: 'assistant',
      content: '',
      created_at: new Date().toISOString(),
      isStreaming: true,
    }
    set(s => ({ messages: [...s.messages, aiMsgPlaceholder] }))

    try {
      const sseHeaders = { 'Content-Type': 'application/json' }
      const sseToken = getToken()
      if (sseToken) sseHeaders['Authorization'] = `Bearer ${sseToken}`

      const res = await fetch(`${BASE}/chats/${chatId}/messages`, {
        method: 'POST',
        headers: sseHeaders,
        body: JSON.stringify({ content, preset_id: presetId || '' }),
      })
      if (!res.ok || !res.body) {
        const body = await res.json().catch(() => null)
        throw new Error(body?.error || `发送失败（HTTP ${res.status}）`)
      }

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
            if (parsed.warning) {
              useUIStore.getState().showToast(parsed.warning, 'warning')
            }
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
      const responseTimeSeconds = Math.max(0, (Date.now() - requestStartedAt) / 1000)
      const normalizedMessages = normalizeChatMessages(freshMessages, fullContent)
      const lastAssistantIndex = normalizedMessages.map(m => m.role).lastIndexOf('assistant')
      const hydratedMessages = normalizedMessages.map((message, index) => (
            index === lastAssistantIndex
              ? { ...message, response_time_seconds: responseTimeSeconds }
              : message
          ))
      set(s => ({
        ...(s.activeChatId === chatId ? { messages: hydratedMessages } : {}),
        streaming: false,
        streamingChatId: null,
        streamKind: null,
        streamBaseSeq: 0,
        streamContent: '',
      }))
    } catch (err) {
      set(s => ({
        ...(s.activeChatId === chatId
          ? { messages: s.messages.filter(m => m.id !== aiMsgPlaceholder.id) }
          : {}),
        streaming: false,
        streamingChatId: null,
        streamKind: null,
        streamBaseSeq: 0,
        streamContent: '',
      }))

      // The server saves the user message before asking the model. Reconcile
      // after a broken connection so the optimistic row becomes the real row.
      try {
        await get().fetchMessages(chatId, { background: true })
      } catch {}
      throw err
    } finally {
      sendingChatIds.delete(chatId)
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

  // 级联删除：删除该消息及其后面的所有消息
  deleteMessageCascade: async (chatId, msgId) => {
    await apiFetch(`/chats/${chatId}/messages/${msgId}`, { method: 'DELETE' })
    // 刷新消息列表
    const data = await apiFetch(`/chats/${chatId}/messages`)
    set({ messages: normalizeChatMessages(data) })
  },

  // 重新生成：后端删除最后一条 AI 回复并重新请求（不重复发送用户消息）
  regenerate: async (chatId) => {
    if (sendingChatIds.has(chatId) || get().streaming) {
      throw chatBusyError()
    }
    sendingChatIds.add(chatId)

    set({
      activeChatId: chatId,
      streaming: true,
      streamingChatId: chatId,
      streamKind: 'regenerate',
      streamBaseSeq: 0,
      streamContent: '',
    })

    const requestStartedAt = Date.now()

    // 先添加一个空的 AI 消息占位
    const aiPlaceholder = {
      id: 'temp-regen-' + Date.now(),
      chat_id: chatId,
      role: 'assistant',
      content: '',
      created_at: new Date().toISOString(),
      isStreaming: true,
    }
    set(s => ({ messages: [...s.messages.filter(m => m.role !== 'assistant' || m !== s.messages[s.messages.length - 1]), aiPlaceholder] }))

    try {
      const sseHeaders = { 'Content-Type': 'application/json' }
      const sseToken = getToken()
      if (sseToken) sseHeaders['Authorization'] = `Bearer ${sseToken}`

      const res = await fetch(`${BASE}/chats/${chatId}/regenerate`, {
        method: 'POST',
        headers: sseHeaders,
      })
      if (!res.ok || !res.body) {
        const body = await res.json().catch(() => null)
        throw new Error(body?.error || `重新生成失败（HTTP ${res.status}）`)
      }

      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let fullContent = ''
      let buffer = ''
      let streamDone = false

      while (!streamDone) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const parts = buffer.split('\n')
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
            if (parsed.warning) {
              useUIStore.getState().showToast(parsed.warning, 'warning')
            }
            if (parsed.token) {
              fullContent += parsed.token
              set(s => ({
                messages: s.messages.map(m =>
                  m.id === aiPlaceholder.id ? { ...m, content: fullContent } : m
                ),
                streamContent: fullContent,
              }))
            }
          } catch (e) {
            if (e.message && !e.message.includes('JSON')) throw e
          }
        }
      }

      const freshMessages = await apiFetch(`/chats/${chatId}/messages`)
      const responseTimeSeconds = Math.max(0, (Date.now() - requestStartedAt) / 1000)
      const normalizedMessages = normalizeChatMessages(freshMessages, fullContent)
      const lastAssistantIndex = normalizedMessages.map(m => m.role).lastIndexOf('assistant')
      const hydratedMessages = normalizedMessages.map((message, index) => (
            index === lastAssistantIndex
              ? { ...message, response_time_seconds: responseTimeSeconds }
              : message
          ))
      set(s => ({
        ...(s.activeChatId === chatId ? { messages: hydratedMessages } : {}),
        streaming: false,
        streamingChatId: null,
        streamKind: null,
        streamBaseSeq: 0,
        streamContent: '',
      }))
    } catch (err) {
      set(s => ({
        ...(s.activeChatId === chatId
          ? { messages: s.messages.filter(m => m.id !== aiPlaceholder.id) }
          : {}),
        streaming: false,
        streamingChatId: null,
        streamKind: null,
        streamBaseSeq: 0,
      }))
      try {
        await get().fetchMessages(chatId, { background: true })
      } catch {}
      throw err
    } finally {
      sendingChatIds.delete(chatId)
    }
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

  fetchEntryTemplates: async () => {
    const data = await apiFetch('/worldbook-templates')
    return (data && data.templates) || []
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
      settings: { ...DEFAULT_SETTINGS },
      loaded: false,

      fetchSettings: async () => {
        try {
          const data = await apiFetch('/settings')
          // 主题用本地存储的值，不被后端覆盖
          const localTheme = localStorage.getItem('litechat-theme')
          if (localTheme) data.theme = localTheme
          // 聊天字号为本地显示偏好，不走后端
          const localFontSize = localStorage.getItem('litechat-chat-font-size')
          const fontSize = localFontSize || get().settings.chat_font_size || DEFAULT_SETTINGS.chat_font_size
          data.chat_font_size = fontSize
          set({ settings: { ...DEFAULT_SETTINGS, ...get().settings, ...data }, loaded: true })
          // 同步主题到 DOM
          const theme = data.theme || 'dark'
          document.documentElement.className = theme
          const meta = document.querySelector('meta[name="theme-color"]')
          if (meta) meta.content = theme === 'light' ? '#f8f9fa' : '#0f0f0f'
          // 同步聊天字号到 CSS 变量
          document.documentElement.style.setProperty('--chat-font-size', fontSize)
        } catch {}
      },

      saveSettings: async (settings) => {
        await apiFetch('/settings', { method: 'PUT', body: settings })
        set({ settings: { ...get().settings, ...settings } })
      },

      setTheme: (theme) => {
        set(s => ({ settings: { ...s.settings, theme } }))
        document.documentElement.className = theme
        // 存到 localStorage（所有用户都能保存自己的主题偏好）
        localStorage.setItem('litechat-theme', theme)
        const meta = document.querySelector('meta[name="theme-color"]')
        if (meta) meta.content = theme === 'light' ? '#f8f9fa' : '#0f0f0f'
      },

      setChatFontSize: (size) => {
        set(s => ({ settings: { ...s.settings, chat_font_size: size } }))
        // 聊天字号是本地显示偏好，立即生效并持久化
        localStorage.setItem('litechat-chat-font-size', size)
        document.documentElement.style.setProperty('--chat-font-size', size)
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
