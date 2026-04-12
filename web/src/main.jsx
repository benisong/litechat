import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { registerSW } from 'virtual:pwa-register'
import App from './App'
import './styles/globals.css'

const updateSW = registerSW({
  immediate: true,
  onNeedRefresh() {
    updateSW(true)
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
