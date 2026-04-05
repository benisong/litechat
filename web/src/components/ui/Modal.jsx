import React, { useEffect } from 'react'
import { X } from 'lucide-react'
import clsx from 'clsx'

export default function Modal({ open, onClose, title, children, className }) {
  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    return () => { document.body.style.overflow = '' }
  }, [open])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* 背景遮罩 */}
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
      />
      {/* 弹窗内容 — 居中显示，限制最大高度留出上下安全区域 */}
      <div className={clsx(
        'relative z-10 w-full max-w-lg mx-auto',
        'bg-dark-50 rounded-2xl',
        'border border-surface-border',
        'max-h-[calc(100dvh-6rem)] overflow-y-auto',
        'animate-slide-up',
        className
      )}>
        {/* 标题栏 — 吸顶 */}
        {title && (
          <div className="flex items-center justify-between px-5 pt-4 pb-2 sticky top-0
                          bg-dark-50 z-10 border-b border-surface-border/50">
            <h2 className="text-lg font-semibold">{title}</h2>
            <button onClick={onClose} className="btn-ghost p-2 -mr-2">
              <X size={20} />
            </button>
          </div>
        )}
        <div className="px-5 pb-5 pt-3">
          {children}
        </div>
      </div>
    </div>
  )
}
