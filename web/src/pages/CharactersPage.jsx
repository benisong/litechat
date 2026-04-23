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
import { useAuthStore, useCharacterStore, useChatStore, useUIStore } from '../store'
import Avatar from '../components/ui/Avatar'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'
import { renderRolePlaceholders } from '../utils/placeholderRender'

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
    title: '选择故事场景',
    subtitle: 'ta 所在的世界，决定了你们能遇见的方式',
    options: [
      { value: 'city', label: '现代都市', desc: '当代城市成年人的生活：写字楼、街区、行业圈层' },
      { value: 'school', label: '校园青春', desc: '当代校园内的青春剧：同学、学长学姐、社团与考试' },
      { value: 'office', label: '都市职场', desc: '公司与行业：同事、上下级、项目合作里的克制拉扯' },
      { value: 'entertainment', label: '娱乐圈', desc: '艺人、经纪、资本与曝光；关系围绕行业活动展开' },
      { value: 'fantasy', label: '西幻异世界', desc: '王国、魔法、种族与骑士秩序（无现代元素）' },
      { value: 'wuxia', label: '仙侠江湖', desc: '门派、修为、剑道与江湖恩怨（古风语气）' },
      { value: 'apocalypse', label: '末日废土', desc: '灾变后世界：据点、幸存者、资源争夺与强绑定' },
    ],
  },
  {
    key: 'type',
    title: '选择关系与基调',
    subtitle: 'ta 和你之间，最适合哪种情绪张力？',
    options: [
      { value: 'pure', label: '心动暧昧', desc: '还没挑明的靠近：慢热、克制、舍不得打破平衡' },
      { value: 'unrequited', label: '求而不得', desc: '存在合理障碍的拉扯：越靠近越心动也越痛' },
      { value: 'healing', label: '治愈陪伴', desc: '已经在一起的稳定关系：日常、互相接住情绪' },
      { value: 'rivalry', label: '欢喜冤家', desc: '地位相近、频繁接触：互怼又默契' },
      { value: 'forbidden', label: '禁忌拉扯', desc: '世界观内合理的身份/立场禁忌：越克制越上头' },
      { value: 'dangerous', label: '危险关系', desc: 'ta 身上带着源自世界观的危险性（视场景不同而落点）' },
    ],
  },
  {
    key: 'personality',
    title: '选择角色性格',
    subtitle: 'ta 的主导性格底色',
    options: [
      { value: 'tsundere', label: '傲娇', desc: '嘴硬心软，嘴上否认但行动很诚实' },
      { value: 'gentle', label: '温柔', desc: '细腻稳定、有原则，不是无限包容' },
      { value: 'scheming', label: '腹黑', desc: '表面从容，实则很会观察、试探和拿捏节奏' },
      { value: 'airhead', label: '天然呆', desc: '反应慢半拍但有自己的判断，无意间就撩到人' },
      { value: 'aloof', label: '高冷', desc: '外冷内热，有距离感，但对偏爱对象会失守' },
      { value: 'dominant', label: '强势', desc: '掌控感强，压迫与保护并存，有具体软化点' },
      { value: 'playful', label: '会撩', desc: '松弛、坏笑、懂得拿捏气氛，有真诚的情感动因' },
    ],
  },
  {
    key: 'pov',
    title: '选择叙事视角',
    subtitle: '你喜欢怎样的开场和代入方式？',
    options: [
      { value: 'second', label: '第二人称', desc: '更沉浸、更贴身，像你就在故事里' },
      { value: 'third', label: '第三人称', desc: '更有画面感，像在旁观一段故事展开' },
    ],
  },
]

function buildGenerationRequest(choices, customPersonality) {
  const [gender, setting, type, personality, pov] = choices
  return {
    gender,
    setting,
    type,
    personality,
    pov,
    custom_personality: customPersonality.trim(),
  }
}

function getChoiceLabels(choices) {
  return choices.map((value, index) => {
    const step = STEPS[index]
    return step?.options.find(option => option.value === value)?.label || value
  })
}

export default function CharactersPage() {
  const navigate = useNavigate()
  const user = useAuthStore(state => state.user)
  const { characters, fetchCharacters, deleteCharacter, generateCharacterCard } = useCharacterStore()
  const { createChat } = useChatStore()
  const { showToast } = useUIStore()

  const [selectedChar, setSelectedChar] = useState(null)
  const [confirmDeleteChar, setConfirmDeleteChar] = useState(null)
  const [showTemplatePrompt, setShowTemplatePrompt] = useState(false)
  const [templateStep, setTemplateStep] = useState(-1)
  const [templateChoices, setTemplateChoices] = useState([])
  const [customPersonality, setCustomPersonality] = useState('')
  const [pendingGenerationChoices, setPendingGenerationChoices] = useState([])
  const [pendingCustomPersonality, setPendingCustomPersonality] = useState('')
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
    setCustomPersonality('')
    setPendingGenerationChoices([])
    setPendingCustomPersonality('')
    setGenerating(false)
  }

  const startTemplateFlow = () => {
    setShowTemplatePrompt(false)
    setTemplateChoices([])
    setCustomPersonality('')
    setPendingGenerationChoices([])
    setPendingCustomPersonality('')
    setTemplateStep(0)
  }

  const handleStepChoice = async value => {
    if (generating) return

    const nextChoices = [...templateChoices, value]
    const isLastStep = templateStep === STEPS.length - 1

    if (!isLastStep) {
      setTemplateChoices(nextChoices)
      setTemplateStep(templateStep + 1)
      return
    }

    setPendingGenerationChoices(nextChoices)
    setPendingCustomPersonality(customPersonality)
    setGenerating(true)
    try {
      const draft = await generateCharacterCard(buildGenerationRequest(nextChoices, customPersonality))
      resetTemplateFlow()
      showToast('角色卡草稿已生成，请确认后保存', 'success')
      navigate('/characters/new', { state: { generatedDraft: draft } })
    } catch (err) {
      setPendingGenerationChoices([])
      setPendingCustomPersonality('')
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
      setPendingCustomPersonality('')
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
            setCustomPersonality('')
            setPendingGenerationChoices([])
            setPendingCustomPersonality('')
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
                  <p className="text-xs text-gray-500 line-clamp-2">
                    {renderRolePlaceholders(char.description, { character: char, user }) || '暂无描述'}
                  </p>
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
                <p className="text-sm text-gray-300 whitespace-pre-wrap">
                  {renderRolePlaceholders(selectedChar.description, { character: selectedChar, user })}
                </p>
              </div>
            )}

            {selectedChar.personality && (
              <div>
                <p className="text-xs text-gray-500 mb-1">性格</p>
                <p className="text-sm text-gray-300 whitespace-pre-wrap">
                  {renderRolePlaceholders(selectedChar.personality, { character: selectedChar, user })}
                </p>
              </div>
            )}

            {selectedChar.scenario && (
              <div>
                <p className="text-xs text-gray-500 mb-1">场景</p>
                <p className="text-sm text-gray-300 whitespace-pre-wrap">
                  {renderRolePlaceholders(selectedChar.scenario, { character: selectedChar, user })}
                </p>
              </div>
            )}

            {selectedChar.first_msg && (
              <div>
                <p className="text-xs text-gray-500 mb-1">开场白</p>
                <p className="text-sm text-gray-300 italic whitespace-pre-wrap">
                  “{renderRolePlaceholders(selectedChar.first_msg, { character: selectedChar, user })}”
                </p>
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
            <button onClick={startTemplateFlow} className="btn-primary w-full py-3 flex items-center justify-center gap-2">
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
              <p className="text-sm text-gray-500 mt-2">AI 正在根据你的模板选项写角色卡</p>
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
            {pendingCustomPersonality.trim() && (
              <div className="rounded-xl border border-surface-border bg-surface/40 p-3 text-left">
                <p className="text-xs text-gray-500 mb-1">性格补充要求</p>
                <p className="text-sm text-gray-300 whitespace-pre-wrap">{pendingCustomPersonality}</p>
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

            {currentStep.key === 'personality' && (
              <div className="rounded-xl border border-surface-border bg-surface/40 p-4 space-y-2">
                <label className="block text-sm text-gray-200">性格补充要求</label>
                <textarea
                  value={customPersonality}
                  onChange={e => setCustomPersonality(e.target.value)}
                  rows={4}
                  className="w-full input-base resize-none text-sm"
                  placeholder="可选，例如：外冷内热、占有欲强、会吃醋、对用户有明显偏爱、说话带一点坏心思。这里写的内容会和你选择的基础性格一起发给 AI。"
                />
                <p className="text-xs text-gray-500">不填也可以，填了之后生成的人设会更贴近你的偏好。</p>
              </div>
            )}

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
