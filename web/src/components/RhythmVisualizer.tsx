import { useMemo } from 'react'
import { Activity, Sparkles, TrendingUp, X } from 'lucide-react'
import type { StyleCard } from '../types'

type Props = {
  body: string
  card: StyleCard | null
  open: boolean
  onClose: () => void
}

type SentenceData = {
  text: string
  length: number
  isDialogue: boolean
}

export default function RhythmVisualizer({ body, card, open, onClose }: Props) {
  const analysis = useMemo(() => {
    const text = body.trim()
    if (!text) {
      return {
        runes: 0,
        dialogueRunes: 0,
        dialogueRatio: 0,
        sentences: [] as SentenceData[],
        avgLength: 0,
        rhythmType: '空白',
        peaks: 0,
        dialogueMatch: 100,
        rhythmMatch: 100,
      }
    }

    const runes = [...text].length

    // Extract dialogue
    const dialogueRegex = /(?:“[^”]*”|「[^」]*」|"[^"]*")/g
    let dialogueRunes = 0
    let match: RegExpExecArray | null
    while ((match = dialogueRegex.exec(text)) !== null) {
      dialogueRunes += [...match[0]].length
    }
    const dialogueRatio = Math.round((dialogueRunes / (runes || 1)) * 100)

    // Split sentences by punctuation
    const rawSentences = text.split(/(?<=[。！？!?\n]+)/)
      .map((s) => s.trim())
      .filter((s) => [...s].length > 0)

    const sentences: SentenceData[] = rawSentences.map((s) => {
      const len = [...s].length
      const isDia = /^([“「"]).*([”」"])$/.test(s) || len < 15 && /[“「]/.test(s)
      return { text: s, length: len, isDialogue: isDia }
    })

    const totalLen = sentences.reduce((sum, s) => sum + s.length, 0)
    const avgLength = sentences.length ? Math.round(totalLen / sentences.length) : 0

    // Count peaks (sentences > avgLength * 1.6)
    const peaks = sentences.filter((s) => s.length > avgLength * 1.5).length

    let rhythmType = '平衡舒缓'
    if (avgLength < 16) {
      rhythmType = '凌厉快打 (短句为主)'
    } else if (avgLength > 38) {
      rhythmType = '沉郁长卷 (铺陈宏大)'
    } else if (dialogueRatio > 55) {
      rhythmType = '机锋对白 (对话主导)'
    } else if (dialogueRatio < 20) {
      rhythmType = '纯粹白描 (叙述主导)'
    }

    // Card alignment index
    let dialogueMatch = 100
    let rhythmMatch = 100
    if (card) {
      const targetDia = card.dimensions?.dialogue_density?.level ?? 50
      dialogueMatch = Math.max(0, 100 - Math.abs(dialogueRatio - targetDia) * 1.2)

      const targetRhythm = card.dimensions?.sentence_rhythm?.level ?? 50
      // High level = shorter sentences, lower level = longer sentences
      const expectedAvg = 45 - (targetRhythm / 100) * 30
      rhythmMatch = Math.max(0, 100 - Math.abs(avgLength - expectedAvg) * 2)
    }

    return {
      runes,
      dialogueRunes,
      dialogueRatio,
      sentences,
      avgLength,
      rhythmType,
      peaks,
      dialogueMatch: Math.round(dialogueMatch),
      rhythmMatch: Math.round(rhythmMatch),
    }
  }, [body, card])

  if (!open) return null

  // SVG Waveform computation
  const svgWidth = 520
  const svgHeight = 120
  const paddingX = 15
  const paddingY = 15

  const points = useMemo(() => {
    const data = analysis.sentences
    if (data.length < 2) return ''
    const maxLen = Math.max(40, ...data.map((d) => d.length))
    const stepX = (svgWidth - paddingX * 2) / (data.length - 1)

    return data
      .map((d, i) => {
        const x = paddingX + i * stepX
        const normalizedY = (d.length / maxLen) * (svgHeight - paddingY * 2)
        const y = svgHeight - paddingY - normalizedY
        return `${x.toFixed(1)},${y.toFixed(1)}`
      })
      .join(' ')
  }, [analysis.sentences])

  const areaPoints = points
    ? `${paddingX},${svgHeight - paddingY} ${points} ${(svgWidth - paddingX).toFixed(1)},${svgHeight - paddingY}`
    : ''

  return (
    <div className="rhythm-visualizer-overlay" onClick={onClose}>
      <div className="rhythm-visualizer-panel" onClick={(e) => e.stopPropagation()}>
        <div className="rhythm-head">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <Activity size={18} color="var(--gold-hi)" />
            <h2 style={{ margin: 0, fontSize: '1.2rem', fontFamily: 'var(--font-serif)' }}>
              行文节奏波形与张力透视仪
            </h2>
          </div>
          <button className="icon-btn" type="button" onClick={onClose} title="关闭透视仪">
            <X size={18} />
          </button>
        </div>

        <div className="rhythm-metrics-grid">
          <div className="rhythm-metric-chip">
            <div className="label">总字符量</div>
            <div className="value">{analysis.runes.toLocaleString()} <span className="unit">字</span></div>
          </div>
          <div className="rhythm-metric-chip">
            <div className="label">平均句长</div>
            <div className="value">{analysis.avgLength} <span className="unit">字/句</span></div>
          </div>
          <div className="rhythm-metric-chip">
            <div className="label">对白占比</div>
            <div className="value" style={{ color: 'var(--jade-hi)' }}>{analysis.dialogueRatio}%</div>
          </div>
          <div className="rhythm-metric-chip">
            <div className="label">主要行文质地</div>
            <div className="value" style={{ fontSize: '0.92rem', color: 'var(--gold-hi)' }}>{analysis.rhythmType}</div>
          </div>
        </div>

        {/* Waveform Section */}
        <div className="rhythm-waveform-box">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.6rem' }}>
            <span style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--ink-soft)', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
              <TrendingUp size={14} color="var(--gold)" />
              长短句呼吸起伏波形 ({analysis.sentences.length} 句)
            </span>
            <span style={{ fontSize: '0.74rem', color: 'var(--ink-faint)', fontFamily: 'var(--mono)' }}>
              波峰 = 长句铺陈 / 波谷 = 紧凑短句
            </span>
          </div>

          {analysis.sentences.length < 3 ? (
            <div style={{ height: '120px', display: 'grid', placeItems: 'center', color: 'var(--ink-faint)', fontSize: '0.85rem' }}>
              输入或生成一段正文后即可实时呈现行文呼吸折线
            </div>
          ) : (
            <svg viewBox={`0 0 ${svgWidth} ${svgHeight}`} className="rhythm-svg">
              <defs>
                <linearGradient id="rhythmAreaGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--gold)" stopOpacity="0.35" />
                  <stop offset="100%" stopColor="var(--gold)" stopOpacity="0.0" />
                </linearGradient>
                <linearGradient id="rhythmLineGrad" x1="0" y1="0" x2="1" y2="0">
                  <stop offset="0%" stopColor="var(--jade-hi)" />
                  <stop offset="50%" stopColor="var(--gold-hi)" />
                  <stop offset="100%" stopColor="var(--cinnabar-hi)" />
                </linearGradient>
              </defs>

              {/* Baseline */}
              <line
                x1={paddingX}
                y1={svgHeight - paddingY}
                x2={svgWidth - paddingX}
                y2={svgHeight - paddingY}
                stroke="rgba(255,255,255,0.1)"
                strokeDasharray="4 4"
              />

              {/* Area Fill */}
              {areaPoints && <polygon points={areaPoints} fill="url(#rhythmAreaGrad)" />}

              {/* Line */}
              {points && (
                <polyline
                  points={points}
                  fill="none"
                  stroke="url(#rhythmLineGrad)"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              )}
            </svg>
          )}
        </div>

        {/* Card Target Alignment */}
        {card && (
          <div className="rhythm-alignment-box">
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', marginBottom: '0.8rem' }}>
              <Sparkles size={16} color="var(--gold-hi)" />
              <span style={{ fontWeight: 600, fontSize: '0.88rem', color: 'var(--ink)' }}>
                与绑定风格卡「{card.name}」参数契合度
              </span>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
              <div className="alignment-bar-wrap">
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.78rem', marginBottom: '0.3rem' }}>
                  <span className="muted">对白密度匹配 (卡片目标 {card.dimensions?.dialogue_density?.level ?? 50})</span>
                  <span style={{ fontWeight: 700, color: 'var(--jade-hi)' }}>{analysis.dialogueMatch}%</span>
                </div>
                <div className="progress-track">
                  <div className="progress-fill" style={{ width: `${analysis.dialogueMatch}%`, background: 'var(--jade-hi)' }} />
                </div>
              </div>

              <div className="alignment-bar-wrap">
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.78rem', marginBottom: '0.3rem' }}>
                  <span className="muted">句式长短匹配 (卡片目标 {card.dimensions?.sentence_rhythm?.level ?? 50})</span>
                  <span style={{ fontWeight: 700, color: 'var(--gold-hi)' }}>{analysis.rhythmMatch}%</span>
                </div>
                <div className="progress-track">
                  <div className="progress-fill" style={{ width: `${analysis.rhythmMatch}%`, background: 'var(--gold-hi)' }} />
                </div>
              </div>
            </div>
          </div>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.2rem' }}>
          <button className="btn sm secondary" type="button" onClick={onClose}>
            完成查阅
          </button>
        </div>
      </div>
    </div>
  )
}
