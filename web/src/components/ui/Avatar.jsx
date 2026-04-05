import React from 'react'
import clsx from 'clsx'

// 根据名字生成渐变颜色
function nameToGradient(name) {
  const gradients = [
    'from-violet-500 to-purple-600',
    'from-blue-500 to-cyan-500',
    'from-pink-500 to-rose-500',
    'from-amber-500 to-orange-500',
    'from-emerald-500 to-teal-500',
    'from-indigo-500 to-blue-600',
    'from-fuchsia-500 to-pink-500',
  ]
  const idx = (name || '?').charCodeAt(0) % gradients.length
  return gradients[idx]
}

export default function Avatar({ name, src, size = 'md', className }) {
  const sizeClasses = {
    xs: 'w-7 h-7 text-xs',
    sm: 'w-9 h-9 text-sm',
    md: 'w-11 h-11 text-base',
    lg: 'w-14 h-14 text-lg',
    xl: 'w-20 h-20 text-2xl',
  }

  if (src) {
    return (
      <img
        src={src}
        alt={name}
        className={clsx(
          'rounded-2xl object-cover flex-shrink-0',
          sizeClasses[size],
          className
        )}
        onError={e => { e.target.style.display = 'none' }}
      />
    )
  }

  return (
    <div className={clsx(
      'rounded-2xl flex items-center justify-center flex-shrink-0',
      `bg-gradient-to-br ${nameToGradient(name)}`,
      sizeClasses[size],
      className
    )}>
      <span className="font-bold text-white">
        {(name || '?')[0].toUpperCase()}
      </span>
    </div>
  )
}
