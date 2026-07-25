import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { registerSW } from 'virtual:pwa-register'
import App from './App'
import { useChatStore } from './store'
import './styles/globals.css'

let updateWaitingForIdle = false

function applyUpdateWhenChatIsIdle(updateSW) {
  if (!useChatStore.getState().streaming) {
    void updateSW(true)
    return
  }
  if (updateWaitingForIdle) return
  updateWaitingForIdle = true

  const unsubscribe = useChatStore.subscribe(state => {
    if (state.streaming) return
    unsubscribe()
    updateWaitingForIdle = false
    void updateSW(true)
  })
}

const updateSW = registerSW({
  immediate: true,
  onNeedRefresh() {
    applyUpdateWhenChatIsIdle(updateSW)
  },
  onRegisteredSW(_swUrl, registration) {
    if (!registration) return

    const checkForUpdate = () => registration.update().catch(() => {})
    checkForUpdate()

    window.addEventListener('focus', checkForUpdate)
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') {
        checkForUpdate()
      }
    })
    window.setInterval(checkForUpdate, 60 * 1000)
  },
})

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </React.StrictMode>
)
