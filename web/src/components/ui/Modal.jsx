import React, { useEffect } from 'react'
import { X } from 'lucide-react'
import clsx from 'clsx'

export default function Modal({ open, onClose, title, children, className }) {
  // 关闭时恢复滚动
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
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center">
      {/* 背景遮罩 */}
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
      />
      {/* 弹窗内容 */}
      <div className={clsx(
        'relative z-10 w-full max-w-lg mx-auto',
        'bg-dark-50 rounded-t-3xl sm:rounded-3xl',
        'border border-surface-border',
        'max-h-[90vh] overflow-y-auto',
        'animate-slide-up',
        className
      )}>
        {/* 标题栏 */}
        {title && (
          <div className="flex items-center justify-between px-5 pt-5 pb-3">
            <h2 className="text-lg font-semibold">{title}</h2>
            <button onClick={onClose} className="btn-ghost p-2 -mr-2">
              <X size={20} />
            </button>
          </div>
        )}
        <div className="px-5 pb-6 pt-2">
          {children}
        </div>
      </div>
    </div>
  )
}
