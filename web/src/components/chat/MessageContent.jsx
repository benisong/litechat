import React, { useMemo, useState } from 'react'
import { marked } from 'marked'
import { ChevronDown, ChevronRight, Brain } from 'lucide-react'
import clsx from 'clsx'

// marked 配置
marked.setOptions({
  breaks: true,    // 换行渲染为 <br>
  gfm: true,       // GitHub 风格 markdown
})

// 需要隐藏的自定义标签（正则匹配整个标签块）
const HIDDEN_TAGS = [
  /<!--[\s\S]*?-->/g,                           // HTML 注释
  /<TBC>[\s\S]*?<\/TBC>/gi,                     // TBC
  /<rule>[\s\S]*?<\/rule>/gi,                    // rule
  /<system>[\s\S]*?<\/system>/gi,                // system
  /<CONFIG>[\s\S]*?<\/CONFIG>/gi,                // CONFIG
  /<AWC>[\s\S]*?<\/AWC>/gi,                      // AWC
  /<ASI>[\s\S]*?<\/ASI>/gi,                      // ASI
  /<STORYTIME>[\s\S]*?<\/STORYTIME>/gi,          // STORYTIME
  /<INTERACTION_MOD>[\s\S]*?<\/INTERACTION_MOD>/gi,
  /<TALKER_MOD>[\s\S]*?<\/TALKER_MOD>/gi,
  /<novelist_MOD>[\s\S]*?<\/novelist_MOD>/gi,
  /<WritingStyle>[\s\S]*?<\/WritingStyle>/gi,
  /<语言风格>[\s\S]*?<\/语言风格>/gi,
]

// 解析思考块：<CoT><details><summary>...</summary>...</details></CoT> 或 <think>...</think>
function parseThinkingBlocks(text) {
  const parts = []
  let remaining = text

  // 匹配 <CoT>...<details><summary>...</summary>...</details>...</CoT>
  const cotRegex = /<CoT>\s*<details>\s*<summary>([\s\S]*?)<\/summary>([\s\S]*?)<\/details>\s*<\/CoT>/gi
  // 匹配 <think>...</think>
  const thinkRegex = /<think>([\s\S]*?)<\/think>/gi

  // 合并两种格式
  const allBlocks = []

  let match
  while ((match = cotRegex.exec(text)) !== null) {
    allBlocks.push({
      index: match.index,
      length: match[0].length,
      title: match[1].trim(),
      content: match[2].trim(),
      full: match[0],
    })
  }
  while ((match = thinkRegex.exec(text)) !== null) {
    allBlocks.push({
      index: match.index,
      length: match[0].length,
      title: '思考过程',
      content: match[1].trim(),
      full: match[0],
    })
  }

  // 按位置排序
  allBlocks.sort((a, b) => a.index - b.index)

  if (allBlocks.length === 0) {
    return [{ type: 'text', content: text }]
  }

  let cursor = 0
  for (const block of allBlocks) {
    if (block.index > cursor) {
      parts.push({ type: 'text', content: remaining.substring(cursor, block.index) })
    }
    parts.push({ type: 'thinking', title: block.title, content: block.content })
    cursor = block.index + block.length
  }
  if (cursor < remaining.length) {
    parts.push({ type: 'text', content: remaining.substring(cursor) })
  }

  return parts
}

// 清理隐藏标签
function cleanHiddenTags(text) {
  let result = text
  for (const regex of HIDDEN_TAGS) {
    result = result.replace(regex, '')
  }
  // 清理多余空行
  result = result.replace(/\n{3,}/g, '\n\n')
  return result.trim()
}

// 渲染 Markdown 文本
function renderMarkdown(text) {
  if (!text) return ''
  const cleaned = cleanHiddenTags(text)
  if (!cleaned) return ''
  return marked.parse(cleaned)
}

// 思考块组件
function ThinkingBlock({ title, content }) {
  const [open, setOpen] = useState(false)

  return (
    <div className="my-2 rounded-lg border border-primary-500/20 bg-primary-500/5 overflow-hidden">
      <button
        onClick={() => setOpen(v => !v)}
        className="w-full flex items-center gap-2 px-3 py-2 text-xs text-primary-300
                   hover:bg-primary-500/10 transition-colors"
      >
        <Brain size={13} className="flex-shrink-0" />
        <span className="font-medium">{title || '思考过程'}</span>
        {open ? <ChevronDown size={13} className="ml-auto" /> : <ChevronRight size={13} className="ml-auto" />}
      </button>
      {open && (
        <div className="px-3 pb-3 text-xs text-gray-400 leading-relaxed whitespace-pre-wrap border-t border-primary-500/10 pt-2">
          {content}
        </div>
      )}
    </div>
  )
}

// 主组件：渲染消息内容
export default function MessageContent({ content, isUser }) {
  const rendered = useMemo(() => {
    if (!content) return null
    if (isUser) {
      // 用户消息：简单渲染，不处理思考块
      return [{ type: 'text', content }]
    }
    return parseThinkingBlocks(content)
  }, [content, isUser])

  if (!rendered) return null

  return (
    <>
      {rendered.map((part, i) => {
        if (part.type === 'thinking') {
          return <ThinkingBlock key={i} title={part.title} content={part.content} />
        }
        // 文本部分：渲染 markdown
        const html = renderMarkdown(part.content)
        if (!html) return null
        return (
          <div
            key={i}
            className="msg-content"
            dangerouslySetInnerHTML={{ __html: html }}
          />
        )
      })}
    </>
  )
}
