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
import ExpandableTextarea from '../components/ui/ExpandableTextarea'
import Modal from '../components/ui/Modal'
import { renderRolePlaceholders } from '../utils/placeholderRender'
import liyuSystemPreset from '../data/liyu-normal-card-v1.json'

const STEPS = [
  {
    key: 'gender',
    title: '选择角色性别',
    subtitle: '性别只决定身份与称谓，不预设性格',
    options: [
      { value: 'female', label: '女生', desc: '女性身份；能力、气质和关系位置不受性别模板限制' },
      { value: 'male', label: '男生', desc: '男性身份；能力、气质和关系位置不受性别模板限制' },
    ],
  },
  {
    key: 'setting',
    title: '选择故事场景',
    subtitle: 'ta 所在的世界，决定了你们能遇见的方式',
    customInput: {
      label: '自定义故事场景',
      placeholder: '例如：一座每逢涨潮就会改变街道布局的海港城，居民依靠记录潮汐来维持日常生活。',
    },
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
    customInput: {
      label: '自定义关系与基调',
      placeholder: '例如：两人是互不信任但必须合作的调查搭档，整体克制、悬疑，关系会随共同选择逐步变化。',
    },
    options: [
      { value: 'pure', label: '心动暧昧', desc: '还没挑明的靠近：慢热、克制、舍不得打破平衡' },
      { value: 'unrequited', label: '求而不得', desc: '真实障碍与选择代价，而不只靠误会维持拉扯' },
      { value: 'healing', label: '治愈陪伴', desc: '互惠而有边界的长期陪伴，不限定是否已经恋爱' },
      { value: 'rivalry', label: '欢喜冤家', desc: '有真实分歧，也有必须合作与逐渐理解的理由' },
      { value: 'forbidden', label: '禁忌拉扯', desc: '规则有来源和后果，人物也能质疑或承担代价' },
      { value: 'dangerous', label: '危险关系', desc: '危险来自立场、秘密或决策，不等同于无边界控制' },
    ],
  },
  {
    key: 'personality',
    title: '选择角色性格',
    subtitle: '选择一种倾向，AI 会补充情境变化与矛盾面',
    customInput: {
      label: '自定义角色性格',
      placeholder: '例如：谨慎寡言，习惯先观察再行动；面对不公时却会冲动介入，事后愿意承担后果。',
    },
    options: [
      { value: 'tsundere', label: '傲娇', desc: '只在特定情境嘴硬，也有坦率可靠的一面' },
      { value: 'gentle', label: '温柔', desc: '主动体贴但保有判断、疲惫和拒绝的边界' },
      { value: 'scheming', label: '腹黑', desc: '擅长局部观察与布局，也会误判并承担代价' },
      { value: 'airhead', label: '天然呆', desc: '注意力顺序独特，有擅长之处而不是缺少常识' },
      { value: 'aloof', label: '高冷', desc: '表达节制、边界清楚，对关系的变化需要积累' },
      { value: 'dominant', label: '强势', desc: '决策果断、愿意负责，但不会替别人决定一切' },
      { value: 'playful', label: '会撩', desc: '懂幽默与距离感，也有认真、笨拙和冷场的时候' },
      { value: 'layered', label: '矛盾混合', desc: '由 AI 组合两种同源却表面冲突的行为倾向' },
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

const DEFAULT_PRESET_SELECTIONS = {
  setting: 'city',
  type: 'pure',
  personality: 'tsundere',
}

const DEFAULT_PRESET_USAGE = {
  setting: true,
  type: true,
  personality: true,
}

const EMPTY_CUSTOM_INPUTS = {
  setting: '',
  type: '',
  personality: '',
}

function buildGenerationRequest(choices, usePresets, customInputs) {
  const [gender, setting, type, personality, pov] = choices
  return {
    gender,
    setting,
    type,
    personality,
    pov,
    use_setting_preset: usePresets.setting,
    use_type_preset: usePresets.type,
    use_personality_preset: usePresets.personality,
    custom_setting: usePresets.setting ? '' : customInputs.setting.trim(),
    custom_type: usePresets.type ? '' : customInputs.type.trim(),
    custom_personality: usePresets.personality ? '' : customInputs.personality.trim(),
  }
}

function getChoiceLabels(choices, usePresets) {
  return choices.map((value, index) => {
    const step = STEPS[index]
    if (step?.customInput && usePresets[step.key] === false) {
      return step.customInput.label
    }
    return step?.options.find(option => option.value === value)?.label || value
  })
}

export default function CharactersPage() {
  const navigate = useNavigate()
  const user = useAuthStore(state => state.user)
  const { characters, fetchCharacters, deleteCharacter, generateCharacterCard, createCharacter } = useCharacterStore()
  const { createChat, createStoryChat } = useChatStore()
  const { showToast } = useUIStore()

  const [selectedChar, setSelectedChar] = useState(null)
  const [confirmDeleteChar, setConfirmDeleteChar] = useState(null)
  const [showTemplatePrompt, setShowTemplatePrompt] = useState(false)
  const [templateStep, setTemplateStep] = useState(-1)
  const [templateChoices, setTemplateChoices] = useState([])
  const [presetSelections, setPresetSelections] = useState({ ...DEFAULT_PRESET_SELECTIONS })
  const [usePresets, setUsePresets] = useState({ ...DEFAULT_PRESET_USAGE })
  const [customInputs, setCustomInputs] = useState({ ...EMPTY_CUSTOM_INPUTS })
  const [pendingGenerationChoices, setPendingGenerationChoices] = useState([])
  const [pendingPresetUsage, setPendingPresetUsage] = useState({ ...DEFAULT_PRESET_USAGE })
  const [pendingCustomInputs, setPendingCustomInputs] = useState({ ...EMPTY_CUSTOM_INPUTS })
  const [generating, setGenerating] = useState(false)
  const [initializingStory, setInitializingStory] = useState(false)

  useEffect(() => {
    fetchCharacters()
  }, [])

  const currentStep = STEPS[templateStep]
  const selectedLabels = useMemo(
    () => getChoiceLabels(templateChoices, usePresets),
    [templateChoices, usePresets]
  )
  const generatingLabels = useMemo(
    () => getChoiceLabels(pendingGenerationChoices, pendingPresetUsage),
    [pendingGenerationChoices, pendingPresetUsage]
  )
  const pendingCustomDetails = useMemo(
    () => STEPS
      .filter(step => step.customInput && pendingPresetUsage[step.key] === false)
      .map(step => ({
        key: step.key,
        label: step.customInput.label,
        value: pendingCustomInputs[step.key],
      })),
    [pendingCustomInputs, pendingPresetUsage]
  )

  const handleStoryChat = async (char, event) => {
    event.stopPropagation()
    if (initializingStory) return
    setInitializingStory(true)
    try {
      const result = await createStoryChat({
        character_id: char.id,
        title: `与${char.name}的复杂剧情`,
        prompt_version: 'story-manifest-v1',
        compile_only_text: [char.description, char.personality, char.scenario].filter(Boolean).join('\n\n'),
      })
      const chat = result.chat || result.Chat
      navigate(`/chats/${chat?.id || chat?.ID}`)
    } catch (err) {
      showToast(err.message || '复杂剧情初始化失败', 'error')
    } finally {
      setInitializingStory(false)
    }
  }

  const handleImportSystemPreset = async () => {
    try {
      const character = await createCharacter({
        name: liyuSystemPreset.name,
        description: liyuSystemPreset.description,
        personality: liyuSystemPreset.personality,
        scenario: liyuSystemPreset.scenario,
        first_msg: liyuSystemPreset.first_msg,
        tags: `${liyuSystemPreset.tags},系统预制,复杂剧情`,
        pov: liyuSystemPreset.pov || 'second',
      })
      setShowTemplatePrompt(false)
      showToast('系统预制角色卡已导入', 'success')
      navigate(`/characters/${character.id}/edit`)
    } catch (err) {
      showToast(err.message || '导入系统预制失败', 'error')
    }
  }

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
    setPresetSelections({ ...DEFAULT_PRESET_SELECTIONS })
    setUsePresets({ ...DEFAULT_PRESET_USAGE })
    setCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
    setPendingGenerationChoices([])
    setPendingPresetUsage({ ...DEFAULT_PRESET_USAGE })
    setPendingCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
    setGenerating(false)
  }

  const openTemplatePrompt = () => {
    setShowTemplatePrompt(true)
    setTemplateStep(-1)
    setTemplateChoices([])
    setPresetSelections({ ...DEFAULT_PRESET_SELECTIONS })
    setUsePresets({ ...DEFAULT_PRESET_USAGE })
    setCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
    setPendingGenerationChoices([])
    setPendingPresetUsage({ ...DEFAULT_PRESET_USAGE })
    setPendingCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
    setGenerating(false)
  }

  const startTemplateFlow = () => {
    setShowTemplatePrompt(false)
    setTemplateChoices([])
    setPresetSelections({ ...DEFAULT_PRESET_SELECTIONS })
    setUsePresets({ ...DEFAULT_PRESET_USAGE })
    setCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
    setPendingGenerationChoices([])
    setPendingPresetUsage({ ...DEFAULT_PRESET_USAGE })
    setPendingCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
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
    setPendingPresetUsage({ ...usePresets })
    setPendingCustomInputs({ ...customInputs })
    setGenerating(true)
    try {
      const draft = await generateCharacterCard(buildGenerationRequest(nextChoices, usePresets, customInputs))
      resetTemplateFlow()
      showToast('角色卡草稿已生成，请确认后保存', 'success')
      navigate('/characters/new', { state: { generatedDraft: draft } })
    } catch (err) {
      setPendingGenerationChoices([])
      setPendingPresetUsage({ ...DEFAULT_PRESET_USAGE })
      setPendingCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
      showToast(err.message || '角色卡生成失败，请重试', 'error')
    } finally {
      setGenerating(false)
    }
  }

  const handleCustomStepContinue = () => {
    if (!currentStep?.customInput || generating) return
    if (!usePresets[currentStep.key] && !customInputs[currentStep.key].trim()) {
      showToast(`请填写${currentStep.customInput.label}`, 'error')
      return
    }
    handleStepChoice(presetSelections[currentStep.key])
  }

  const handleStepBack = () => {
    if (generating) return
    if (templateStep <= 0) {
      setTemplateStep(-1)
      setTemplateChoices([])
      setPendingGenerationChoices([])
      setPendingPresetUsage({ ...DEFAULT_PRESET_USAGE })
      setPendingCustomInputs({ ...EMPTY_CUSTOM_INPUTS })
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
          onClick={openTemplatePrompt}
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
            action={<button onClick={openTemplatePrompt} className="btn-primary">创建角色</button>}
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
                  {char.tags?.includes('复杂剧情') && (
                    <button
                      onClick={e => handleStoryChat(char, e)}
                      disabled={initializingStory}
                      className="flex-1 flex items-center justify-center gap-1.5 py-2 rounded-xl bg-amber-500/15 text-amber-300 text-xs font-medium hover:bg-amber-500/25 transition-colors disabled:cursor-wait disabled:opacity-60"
                    >
                      {initializingStory ? <Loader2 size={13} className="animate-spin" /> : <Sparkles size={13} />}
                      {initializingStory ? '初始化中…' : '剧情'}
                    </button>
                  )}
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
            <p className="text-xs text-gray-500 mt-1">选择预设或填写自定义方向，再交给 AI 生成角色卡草稿</p>
          </div>
          <div className="flex flex-col gap-3">
            <button onClick={startTemplateFlow} className="btn-primary w-full py-3 flex items-center justify-center gap-2">
              <Sparkles size={16} />
              使用模板生成
            </button>
            <button
              onClick={handleImportSystemPreset}
              className="w-full py-3 rounded-xl border border-primary-500/40 bg-primary-500/10 text-primary-200 hover:bg-primary-500/20 transition-colors text-sm flex items-center justify-center gap-2"
            >
              <Sparkles size={16} />
              导入系统预制：李预·复杂剧情
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
              <p className="text-sm text-gray-500 mt-2">AI 正在根据你的预设与自定义要求写角色卡</p>
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
            {pendingCustomDetails.map(detail => (
              <div key={detail.key} className="rounded-xl border border-surface-border bg-surface/40 p-3 text-left">
                <p className="text-xs text-gray-500 mb-1">{detail.label}</p>
                <p className="text-sm text-gray-300 whitespace-pre-wrap">{detail.value}</p>
              </div>
            ))}
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

            {currentStep.customInput ? (
              <div className="space-y-4 rounded-2xl border border-surface-border bg-surface/30 p-4">
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <label htmlFor={`${currentStep.key}-preset`} className="text-sm font-medium text-gray-200">
                      选择预设
                    </label>
                    <label className="flex cursor-pointer items-center gap-2 text-xs text-gray-400">
                      <input
                        type="checkbox"
                        checked={usePresets[currentStep.key]}
                        onChange={event => setUsePresets(current => ({
                          ...current,
                          [currentStep.key]: event.target.checked,
                        }))}
                        className="h-4 w-4 accent-primary-500"
                      />
                      使用预设
                    </label>
                  </div>
                  <select
                    id={`${currentStep.key}-preset`}
                    value={presetSelections[currentStep.key]}
                    onChange={event => setPresetSelections(current => ({
                      ...current,
                      [currentStep.key]: event.target.value,
                    }))}
                    disabled={!usePresets[currentStep.key]}
                    className="input-base w-full bg-surface text-sm disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    {currentStep.options.map(option => (
                      <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                  </select>
                  {usePresets[currentStep.key] && (
                    <p className="text-xs leading-5 text-gray-500">
                      {currentStep.options.find(option => option.value === presetSelections[currentStep.key])?.desc}
                    </p>
                  )}
                </div>

                <div className="space-y-2">
                  <label htmlFor={`${currentStep.key}-custom`} className="block text-sm font-medium text-gray-200">
                    自定义输入
                  </label>
                  <ExpandableTextarea
                    id={`${currentStep.key}-custom`}
                    editorTitle={currentStep.customInput.label}
                    value={customInputs[currentStep.key]}
                    onChange={event => setCustomInputs(current => ({
                      ...current,
                      [currentStep.key]: event.target.value,
                    }))}
                    disabled={usePresets[currentStep.key]}
                    rows={5}
                    className="input-base w-full resize-none text-sm disabled:cursor-not-allowed disabled:bg-dark-200/60 disabled:text-gray-600"
                    placeholder={currentStep.customInput.placeholder}
                  />
                  <p className="text-xs text-gray-500">
                    {usePresets[currentStep.key]
                      ? '取消勾选“使用预设”后即可填写自定义内容。'
                      : '当前只采用自定义内容，不会同时叠加下拉框中的预设。'}
                  </p>
                </div>

                <button onClick={handleCustomStepContinue} className="btn-primary w-full py-2.5 text-sm">
                  下一步
                </button>
              </div>
            ) : (
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
