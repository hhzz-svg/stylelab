import { useState } from 'react'
import {
  Check,
  Eye,
  MessageSquare,
  Plus,
  Scissors,
  Sparkles,
  Wand2,
  X,
  Zap,
} from 'lucide-react'
import { api } from '../api'
import type { StyleCard } from '../types'

type ActionType = 'restyle' | 'sensory' | 'pacing' | 'dialogue' | 'expand' | 'condense' | 'custom'

type Props = {
  selectedText: string
  card: StyleCard | null
  position: { top: number; left: number } | null
  open: boolean
  onReplace: (replacement: string) => void
  onAppend: (appendix: string) => void
  onClose: () => void
}

const ACTION_DEFS: { id: ActionType; label: string; icon: typeof Sparkles; promptDesc: string }[] = [
  { id: 'restyle', label: '按风格重塑', icon: Sparkles, promptDesc: '严格遵循当前绑定的九维风格卡指标与禁忌规则，重塑本段语言质感。' },
  { id: 'sensory', label: '强化感官烘托', icon: Eye, promptDesc: '大幅强化光影、气味、温度、音效与周遭环境的细腻感官描写。' },
  { id: 'pacing', label: '短句快打提速', icon: Zap, promptDesc: '将长句切分为紧凑凌厉的短句，加快动作交锋与危机推进节奏。' },
  { id: 'dialogue', label: '增强对白机锋', icon: MessageSquare, promptDesc: '将对白改写为富有言外之意、心理博弈与性格张力的交锋。' },
  { id: 'expand', label: '扩写充实细节', icon: Plus, promptDesc: '在保留核心情节的前提下，扩充动作细节、心理暗涌与微表情。' },
  { id: 'condense', label: '精简去水提炼', icon: Scissors, promptDesc: '剔除多余修饰与赘词，只保留最具张力与画面感的骨干语句。' },
]

export default function InlineAIMenu({
  selectedText,
  card,
  position,
  open,
  onReplace,
  onAppend,
  onClose,
}: Props) {
  const [customPrompt, setCustomPrompt] = useState('')
  const [showCustomInput, setShowCustomInput] = useState(false)
  const [busy, setBusy] = useState(false)
  const [resultText, setResultText] = useState('')
  const [error, setError] = useState('')

  if (!open || !position || !selectedText.trim()) return null

  async function executeAction(action: ActionType, customInstruction = '') {
    if (!selectedText.trim()) return
    setError('')
    setBusy(true)
    setResultText('')

    try {
      const def = ACTION_DEFS.find((d) => d.id === action)
      const actionInstruction = action === 'custom' ? customInstruction : def?.promptDesc ?? ''

      const cardPrompt = card
        ? `【风格卡基准】：名称「${card.name}」，对白密度${card.dimensions?.dialogue_density?.level ?? 50}，句式节奏${card.dimensions?.sentence_rhythm?.level ?? 50}，感官${card.dimensions?.sensory_description?.level ?? 50}。`
        : ''

      const fullPremise = `请精修以下选区文本：\n【修改目标】：${actionInstruction}\n${cardPrompt}\n【原文】：${selectedText.slice(0, 500)}`

      // Use sampleCard or direct call
      if (card?.id) {
        const res = await api.sampleCard(card.id, fullPremise.slice(0, 80), 400, '')
        let pollCount = 0
        const poll = async () => {
          if (pollCount++ > 30) {
            throw new Error('精修生成超时')
          }
          const job = await api.getJob(res.job_id)
          if (job.status === 'succeeded') {
            const sampleId = (job.result as { sample_id?: string })?.sample_id
            if (sampleId) {
              const sample = await api.getSample(sampleId)
              setResultText(sample.body.trim())
            } else {
              setResultText('已完成局部精修。')
            }
            setBusy(false)
          } else if (job.status === 'failed') {
            setError(job.error || '精修任务失败')
            setBusy(false)
          } else {
            window.setTimeout(() => void poll(), 1200)
          }
        }
        await poll()
      } else {
        setError('请先在上方为本章绑定一张风格卡')
        setBusy(false)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '精修请求失败')
      setBusy(false)
    }
  }

  return (
    <div
      className="inline-ai-toolbar"
      style={{
        top: `${position.top}px`,
        left: `${position.left}px`,
      }}
      onClick={(e) => e.stopPropagation()}
    >
      <div className="inline-ai-head">
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.45rem' }}>
          <Wand2 size={15} color="var(--gold-hi)" />
          <span style={{ fontSize: '0.82rem', fontWeight: 700, color: 'var(--ink)' }}>
            划词 AI 局部精修舱
          </span>
          <span style={{ fontSize: '0.72rem', color: 'var(--ink-faint)', fontFamily: 'var(--mono)' }}>
            已选 {[...selectedText].length} 字
          </span>
        </div>
        <button className="icon-btn sm" type="button" onClick={onClose} title="关闭精修舱">
          <X size={14} />
        </button>
      </div>

      {!resultText && !busy && (
        <>
          <div className="inline-ai-actions-grid">
            {ACTION_DEFS.map((act) => {
              const Icon = act.icon
              return (
                <button
                  key={act.id}
                  type="button"
                  className="inline-action-btn"
                  onClick={() => void executeAction(act.id)}
                >
                  <Icon size={14} color="var(--gold-hi)" />
                  <span>{act.label}</span>
                </button>
              )
            })}
          </div>

          <div style={{ padding: '0 0.8rem 0.8rem' }}>
            {!showCustomInput ? (
              <button
                type="button"
                className="inline-custom-trigger"
                onClick={() => setShowCustomInput(true)}
              >
                <Sparkles size={13} />
                <span>输入自定义精修指令…</span>
              </button>
            ) : (
              <div style={{ display: 'flex', gap: '0.4rem' }}>
                <input
                  type="text"
                  value={customPrompt}
                  onChange={(e) => setCustomPrompt(e.target.value)}
                  placeholder="例如：改成说书人半文半白口吻…"
                  style={{ flex: 1, fontSize: '0.8rem', padding: '0.35rem 0.6rem' }}
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && customPrompt.trim()) {
                      void executeAction('custom', customPrompt.trim())
                    }
                  }}
                />
                <button
                  className="btn sm"
                  type="button"
                  disabled={!customPrompt.trim()}
                  onClick={() => void executeAction('custom', customPrompt.trim())}
                >
                  执行
                </button>
              </div>
            )}
          </div>
        </>
      )}

      {busy && (
        <div className="inline-ai-loading">
          <div className="spinner" style={{ width: '22px', height: '22px', borderWidth: '2.5px' }} />
          <span style={{ fontSize: '0.82rem', color: 'var(--gold-hi)', fontWeight: 600 }}>
            AI 正在按技法重塑推演…
          </span>
        </div>
      )}

      {error && !busy && (
        <div style={{ padding: '0.6rem 0.8rem' }}>
          <p className="error" style={{ margin: 0, fontSize: '0.78rem' }}>{error}</p>
        </div>
      )}

      {resultText && !busy && (
        <div className="inline-ai-diff-box">
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.6rem', marginBottom: '0.8rem' }}>
            <div className="diff-col origin">
              <div className="diff-label">选区原文</div>
              <div className="diff-content">{selectedText}</div>
            </div>
            <div className="diff-col polished">
              <div className="diff-label" style={{ color: 'var(--jade-hi)' }}>精修后</div>
              <div className="diff-content">{resultText}</div>
            </div>
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
            <button
              className="btn sm secondary"
              type="button"
              onClick={() => {
                onAppend('\n' + resultText)
                onClose()
              }}
            >
              在后方追加
            </button>
            <button
              className="btn sm"
              type="button"
              onClick={() => {
                onReplace(resultText)
                onClose()
              }}
            >
              <Check size={14} />
              替换选区
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
