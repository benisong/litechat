import React, { useEffect, useId, useRef, useState } from 'react'
import { ArrowLeft, Maximize2 } from 'lucide-react'
import { createPortal } from 'react-dom'
import clsx from 'clsx'

export default function ExpandableTextarea({
  className,
  disabled = false,
  editorTitle = '大屏编辑',
  expandedClassName,
  id,
  name,
  rows = 4,
  wrapperClassName,
  ...textareaProps
}) {
  const generatedId = useId()
  const expandedTextareaRef = useRef(null)
  const [expanded, setExpanded] = useState(false)
  const textareaId = id || `expandable-textarea-${generatedId.replace(/:/g, '')}`

  useEffect(() => {
    if (!expanded) return undefined

    const previousOverflow = document.body.style.overflow
    const closeOnEscape = event => {
      if (event.key === 'Escape') setExpanded(false)
    }

    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', closeOnEscape)
    const frame = window.requestAnimationFrame(() => {
      const textarea = expandedTextareaRef.current
      if (!textarea) return
      textarea.focus()
      const end = textarea.value.length
      textarea.setSelectionRange(end, end)
    })

    return () => {
      window.cancelAnimationFrame(frame)
      window.removeEventListener('keydown', closeOnEscape)
      // A parent modal may have restored scrolling first during unmount.
      if (document.body.style.overflow === 'hidden') {
        document.body.style.overflow = previousOverflow
      }
    }
  }, [expanded])

  useEffect(() => {
    if (disabled && expanded) setExpanded(false)
  }, [disabled, expanded])

  const expandedEditor = expanded && typeof document !== 'undefined'
    ? createPortal(
        <div
          className="fixed inset-0 z-[100] flex h-[100dvh] flex-col bg-dark-100/95 backdrop-blur-xl"
          role="dialog"
          aria-modal="true"
          aria-labelledby={`${textareaId}-expanded-title`}
        >
          <div className="flex items-center justify-between gap-4 border-b border-surface-border px-4 pb-3 pt-[calc(0.75rem+env(safe-area-inset-top))] sm:px-6">
            <div className="min-w-0">
              <p id={`${textareaId}-expanded-title`} className="truncate text-base font-semibold text-gray-100">
                {editorTitle}
              </p>
              <p className="mt-0.5 text-xs text-gray-500">内容会实时同步到原输入框</p>
            </div>
            <button
              type="button"
              onClick={() => setExpanded(false)}
              className="btn-ghost flex shrink-0 items-center gap-1.5 px-3 py-2 text-sm"
              aria-label="返回原页面"
            >
              <ArrowLeft size={17} />
              返回
            </button>
          </div>
          <div className="flex min-h-0 flex-1 p-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] sm:p-6">
            <textarea
              {...textareaProps}
              ref={expandedTextareaRef}
              id={`${textareaId}-expanded`}
              disabled={disabled}
              className={clsx(
                'input-base min-h-0 w-full flex-1 resize-none rounded-2xl p-4 text-base leading-7 sm:p-6',
                expandedClassName
              )}
            />
          </div>
        </div>,
        document.body
      )
    : null

  return (
    <>
      <div className={clsx('relative', wrapperClassName)}>
        <textarea
          {...textareaProps}
          id={textareaId}
          name={name}
          rows={rows}
          disabled={disabled}
          className={clsx('pr-11', className)}
        />
        <button
          type="button"
          onClick={() => setExpanded(true)}
          disabled={disabled}
          className={clsx(
            'absolute right-2 top-2 rounded-lg border border-surface-border bg-dark-100/90 p-2 text-gray-400 shadow-lg transition-colors',
            disabled
              ? 'cursor-not-allowed opacity-30'
              : 'hover:border-primary-500/50 hover:text-primary-300'
          )}
          title="放大编辑"
          aria-label={`${editorTitle}，放大编辑`}
        >
          <Maximize2 size={15} />
        </button>
      </div>
      {expandedEditor}
    </>
  )
}
