import React from 'react'
import { useUIStore } from '../../store'
import { CheckCircle, AlertCircle, Info, X } from 'lucide-react'
import clsx from 'clsx'

const ICONS = {
  success: CheckCircle,
  error:   AlertCircle,
  info:    Info,
}

const COLORS = {
  success: 'border-green-500/30 bg-green-500/10 text-green-400',
  error:   'border-red-500/30 bg-red-500/10 text-red-400',
  info:    'border-primary-500/30 bg-primary-500/10 text-primary-400',
}

export default function Toast() {
  const { toast } = useUIStore()
  if (!toast) return null

  const Icon = ICONS[toast.type] || Info

  return (
    <div className="fixed top-4 left-1/2 -translate-x-1/2 z-[100] animate-slide-up">
      <div className={clsx(
        'flex items-center gap-2 px-4 py-3 rounded-xl border glass text-sm font-medium shadow-lg',
        COLORS[toast.type] || COLORS.info
      )}>
        <Icon size={16} />
        <span className="text-white">{toast.message}</span>
      </div>
    </div>
  )
}
