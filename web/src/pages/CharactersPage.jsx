import React, { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Users,
  Plus,
  MessageSquare,
  Edit2,
  Trash2,
  Sparkles,
  ArrowLeft,
  Loader2,
} from 'lucide-react'
import { useCharacterStore, useChatStore, useUIStore } from '../store'
import Avatar from '../components/ui/Avatar'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'

const STEPS = [
  {
    key: 'gender',
    title: '选择角色性别',
    subtitle: '你想遇见怎样的 ta？',
    options: [
      { value: 'female', label: '女生', desc: '温柔、明媚、让人心动的女性角色' },
      { value: 'male', label: '男生', desc: '温暖、沉稳、让人安心的男性角色' },
    ],
  },
  {
    key: 'setting',
    title: '选择故事舞台',
    subtitle: '你们的故事发生在哪里？',
    options: [
      { value: 'city', label: '都市', desc: '写字楼、咖啡厅、深夜地铁里的成年人心动' },
      { value: 'school', label: '校园', desc: '教室、操场、放学后小路上的青春悸动' },
    ],
  },
  {
    key: 'type',
    title: '选择故事基调',
    subtitle: '你更想要什么样的感觉？',
    options: [
      { value: 'pure', label: '白月光', desc: '温柔治愈、暧昧日常、适合慢慢相处' },
      { value: 'unrequited', label: '求而不得', desc: '克制拉扯、若即若离、张力更强' },
    ],
  },
  {
    key: 'personality',
    title: '选择角色性格',
    subtitle: 'ta 会是什么样的人？',
    options: [
      { value: 'tsundere', label: '傲娇', desc: '嘴上说着不在意，行动却很诚实' },
      { value: 'gentle', label: '温柔', desc: '像春天的风，细腻、稳定、会照顾人' },
      { value: 'scheming', label: '腹黑', desc: '笑得很好看，但总让人猜不透心思' },
      { value: 'airhead', label: '天然呆', desc: '反应慢半拍，却总能无意间撩到人' },
    ],
  },
  {
    key: 'pov',
    title: '选择叙事视角',
    subtitle: '你喜欢怎样的叙事方式？',
    options: [
      { value: 'second', label: '第二人称', desc: '更沉浸、更贴身，像你就在故事里' },
      { value: 'third', label: '第三人称', desc: '更有画面感，像在旁观一段故事展开' },
    ],
  },
]

function buildGenerationRequest(choices) {
  const [gender, setting, type, personality, pov] = choices
  return { gender, setting, type, personality, pov }
}

function getChoiceLabels(choices) {
  return choices.map((value, index) => {
    const step = STEPS[index]
    return step?.options.find(option => option.value === value)?.label || value
  })
}

export default function CharactersPage() {
  const navigate = useNavigate()
  const { characters, fetchCharacters, deleteCharacter, generateCharacterCard } = useCharacterStore()
  const { createChat } = useChatStore()
  const { showToast } = useUIStore()

  const [selectedChar, setSelectedChar] = useState(null)
  const [confirmDeleteChar, setConfirmDeleteChar] = useState(null)
  const [showTemplatePrompt, setShowTemplatePrompt] = useState(false)
  const [templateStep, setTemplateStep] = useState(-1)
  const [templateChoices, setTemplateChoices] = useState([])
  const [pendingGenerationChoices, setPendingGenerationChoices] = useState([])
  const [generating, setGenerating] = useState(false)

  useEffect(() => {
    fetchCharacters()
  }, [])

  const currentStep = STEPS[templateStep]
  const selectedLabels = useMemo(() => getChoiceLabels(templateChoices), [templateChoices])
  const generatingLabels = useMemo(() => getChoiceLabels(pendingGenerationChoices), [pendingGenerationChoices])

  const handleChat = async (char, event) => {
    event.stopPropagation()
    try {
      const chat = await createChat(char.id, `与${char.name}的对话`)
      navigate(`/chats/${chat.id}`)
    } catch {
      showToast('创建对话失败', 'error')
    }
  }

  const handleDeleteConfirm = async () => {
    if (!confirmDeleteChar) return
    try {
      await deleteCharacter(confirmDeleteChar.id)
      useChatStore.getState().fetchChats()
      showToast('角色已删除', 'success')
    } catch {
      showToast('删除失败', 'error')
    } finally {
      setConfirmDeleteChar(null)
    }
  }

  const resetTemplateFlow = () => {
    setShowTemplatePrompt(false)
    setTemplateStep(-1)
    setTemplateChoices([])
    setPendingGenerationChoices([])
    setGenerating(false)
  }

  const handleUseTemplate = () => {
    setShowTemplatePrompt(false)
    setTemplateChoices([])
    setPendingGenerationChoices([])
    setTemplateStep(0)
  }

  const handleStepChoice = async (value) => {
    if (generating) return

    const nextChoices = [...templateChoices, value]
    const isLastStep = templateStep === STEPS.length - 1

    if (!isLastStep) {
      setTemplateChoices(nextChoices)
      setTemplateStep(templateStep + 1)
      return
    }

    setPendingGenerationChoices(nextChoices)
    setGenerating(true)
    try {
      const draft = await generateCharacterCard(buildGenerationRequest(nextChoices))
      resetTemplateFlow()
      showToast('角色卡草稿已生成，请确认后保存', 'success')
      navigate('/characters/new', { state: { generatedDraft: draft } })
    } catch (err) {
      setPendingGenerationChoices([])
      showToast(err.message || '角色卡生成失败，请重试', 'error')
    } finally {
      setGenerating(false)
    }
  }

  const handleStepBack = () => {
    if (generating) return
    if (templateStep <= 0) {
      setTemplateStep(-1)
      setTemplateChoices([])
      setPendingGenerationChoices([])
      setShowTemplatePrompt(true)
      return
    }
    setTemplateChoices(prev => prev.slice(0, -1))
    setTemplateStep(prev => prev - 1)
  }

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 pt-12 pb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold">角色</h1>
        <button
          onClick={() => {
            setShowTemplatePrompt(true)
            setTemplateStep(-1)
            setTemplateChoices([])
            setPendingGenerationChoices([])
          }}
          className="btn-primary flex items-center gap-2 py-2 px-4 text-sm"
        >
          <Plus size={16} />
          新建
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-4">
        {characters.length === 0 ? (
          <EmptyState
            icon={Users}
            title="还没有角色卡"
            description="创建你的第一个 AI 角色"
            action={<button onClick={() => setShowTemplatePrompt(true)} className="btn-primary">创建角色</button>}
          />
        ) : (
          <div className="grid grid-cols-2 gap-3 pb-4">
            {characters.map(char => (
              <div
                key={char.id}
                className="card p-4 flex flex-col gap-3 cursor-pointer hover:bg-surface-hover active:scale-[0.98] transition-all duration-150"
                onClick={() => setSelectedChar(char)}
              >
                <div className="flex items-start justify-between">
                  <Avatar name={char.name} src={char.avatar_url} size="lg" />
                  {char.tags && (
                    <span className="text-[10px] bg-primary-500/20 text-primary-300 px-2 py-0.5 rounded-full border border-primary-500/20">
                      {char.tags.split(',')[0]}
                    </span>
                  )}
                </div>

                <div>
                  <h3 className="font-semibold text-sm mb-1 truncate">{char.name}</h3>
                  <p className="text-xs text-gray-500 line-clamp-2">{char.description || '暂无描述'}</p>
                </div>

                <div className="flex gap-2 mt-auto">
                  <button
                    onClick={e => handleChat(char, e)}
                    className="flex-1 flex items-center justify-center gap-1.5 py-2 rounded-xl bg-primary-600/20 text-primary-400 text-xs font-medium hover:bg-primary-600/30 transition-colors"
                  >
                    <MessageSquare size={13} />
                    聊天
                  </button>
                  <button
                    onClick={e => {
                      e.stopPropagation()
                      navigate(`/characters/${char.id}/edit`)
                    }}
                    className="p-2 rounded-xl bg-surface-hover text-gray-400 hover:text-white transition-colors"
                  >
                    <Edit2 size={14} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <Modal open={!!selectedChar} onClose={() => setSelectedChar(null)} title={selectedChar?.name}>
        {selectedChar && (
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              <Avatar name={selectedChar.name} src={selectedChar.avatar_url} size="xl" />
              <div>
                <h3 className="text-xl font-bold">{selectedChar.name}</h3>
                {selectedChar.tags && (
                  <div className="flex gap-1 mt-1 flex-wrap">
                    {selectedChar.tags.split(',').map(tag => (
                      <span key={tag} className="text-xs bg-surface px-2 py-0.5 rounded-full text-gray-400 border border-surface-border">
                        {tag.trim()}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {selectedChar.description && (
              <div>
                <p className="text-xs text-gray-500 mb-1">描述</p>
                <p className="text-sm text-gray-300">{selectedChar.description}</p>
              </div>
            )}

            {selectedChar.personality && (
              <div>
                <p className="text-xs text-gray-500 mb-1">性格</p>
                <p className="text-sm text-gray-300">{selectedChar.personality}</p>
              </div>
            )}

            {selectedChar.first_msg && (
              <div>
                <p className="text-xs text-gray-500 mb-1">开场白</p>
                <p className="text-sm text-gray-300 italic">“{selectedChar.first_msg}”</p>
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <button
                onClick={e => {
                  setSelectedChar(null)
                  handleChat(selectedChar, e)
                }}
                className="flex-1 btn-primary flex items-center justify-center gap-2"
              >
                <MessageSquare size={16} />
                开始聊天
              </button>
              <button
                onClick={() => {
                  setSelectedChar(null)
                  navigate(`/characters/${selectedChar.id}/edit`)
                }}
                className="px-4 py-2.5 rounded-xl border border-surface-border text-gray-300 hover:bg-surface-hover transition-colors"
              >
                <Edit2 size={16} />
              </button>
              <button
                onClick={() => {
                  setConfirmDeleteChar(selectedChar)
                  setSelectedChar(null)
                }}
                className="px-4 py-2.5 rounded-xl border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors"
              >
                <Trash2 size={16} />
              </button>
            </div>
          </div>
        )}
      </Modal>

      <Modal open={!!confirmDeleteChar} onClose={() => setConfirmDeleteChar(null)} title="确认删除">
        {confirmDeleteChar && (
          <div className="space-y-4">
            <p className="text-sm text-gray-300">确定要删除角色“{confirmDeleteChar.name}”吗？</p>
            <p className="text-xs text-red-400">删除后会同时删除该角色的所有对话和消息，此操作不可恢复。</p>
            <div className="flex gap-3 pt-2">
              <button
                onClick={() => setConfirmDeleteChar(null)}
                className="flex-1 py-2.5 rounded-xl border border-surface-border text-gray-300 hover:bg-surface-hover transition-colors text-sm"
              >
                取消
              </button>
              <button
                onClick={handleDeleteConfirm}
                className="flex-1 py-2.5 rounded-xl bg-red-600 text-white text-sm hover:bg-red-700 transition-colors"
              >
                确认删除
              </button>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        open={showTemplatePrompt}
        onClose={() => !generating && setShowTemplatePrompt(false)}
        title="创建角色卡"
      >
        <div className="space-y-4">
          <div className="text-center py-2">
            <Sparkles size={32} className="mx-auto mb-3 text-primary-400" />
            <p className="text-sm text-gray-300">想快速生成一张角色卡吗？</p>
            <p className="text-xs text-gray-500 mt-1">先完成模板选择，再交给 AI 生成角色卡草稿</p>
          </div>
          <div className="flex flex-col gap-3">
            <button onClick={handleUseTemplate} className="btn-primary w-full py-3 flex items-center justify-center gap-2">
              <Sparkles size={16} />
              使用模板生成
            </button>
            <button
              onClick={() => {
                setShowTemplatePrompt(false)
                navigate('/characters/new')
              }}
              className="w-full py-3 rounded-xl border border-surface-border text-gray-400 hover:bg-surface-hover transition-colors text-sm"
            >
              自己创建
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={templateStep >= 0}
        onClose={() => {
          if (!generating) resetTemplateFlow()
        }}
        title={generating ? '生成角色卡' : currentStep?.title}
      >
        {generating ? (
          <div className="py-8 space-y-5 text-center">
            <Loader2 size={32} className="mx-auto text-primary-400 animate-spin" />
            <div>
              <p className="text-base font-medium text-gray-100">生成角色卡中，请等候</p>
              <p className="text-sm text-gray-500 mt-2">AI 正在根据你的模板选择生成角色卡内容</p>
            </div>
            {generatingLabels.length > 0 && (
              <div className="flex flex-wrap justify-center gap-2">
                {generatingLabels.map(label => (
                  <span key={label} className="px-2.5 py-1 rounded-full text-xs bg-primary-500/10 border border-primary-500/20 text-primary-300">
                    {label}
                  </span>
                ))}
              </div>
            )}
          </div>
        ) : currentStep ? (
          <div className="space-y-4">
            <div className="flex items-center gap-1.5 justify-center">
              {STEPS.map((_, i) => (
                <div
                  key={i}
                  className={`h-1.5 rounded-full transition-all duration-300 ${
                    i < templateStep
                      ? 'w-6 bg-primary-500'
                      : i === templateStep
                        ? 'w-6 bg-primary-400 animate-pulse'
                        : 'w-6 bg-surface-border'
                  }`}
                />
              ))}
            </div>

            <p className="text-center text-sm text-gray-400">{currentStep.subtitle}</p>

            {selectedLabels.length > 0 && (
              <div className="flex flex-wrap gap-2 justify-center">
                {selectedLabels.map(label => (
                  <span key={label} className="px-2.5 py-1 rounded-full text-xs bg-surface border border-surface-border text-gray-300">
                    {label}
                  </span>
                ))}
              </div>
            )}

            <div className={`gap-3 ${currentStep.options.length > 2 ? 'grid grid-cols-2' : 'flex flex-col'}`}>
              {currentStep.options.map(option => (
                <button
                  key={option.value}
                  onClick={() => handleStepChoice(option.value)}
                  className="w-full text-left p-4 rounded-xl border border-surface-border hover:border-primary-500/50 hover:bg-primary-600/10 active:scale-[0.98] transition-all duration-150"
                >
                  <span className="text-base font-medium text-gray-200">{option.label}</span>
                  <p className="text-xs text-gray-500 mt-1">{option.desc}</p>
                </button>
              ))}
            </div>

            <button
              onClick={handleStepBack}
              className="w-full flex items-center justify-center gap-1.5 py-2.5 text-sm text-gray-500 hover:text-gray-300 transition-colors"
            >
              <ArrowLeft size={14} />
              返回上一步
            </button>
          </div>
        ) : null}
      </Modal>
    </div>
  )
}
