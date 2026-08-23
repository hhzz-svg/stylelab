import { Minus, Plus } from 'lucide-react'
import { DIMENSION_KEYS, DIMENSION_LABELS, type Dimension, type DimensionKey } from '../types'

type Props = {
  levels: Record<string, number>
  dimensions?: Record<string, Dimension>
  onChange?: (key: DimensionKey, value: number) => void
  disabled?: boolean
  focus?: DimensionKey
  onFocus?: (key: DimensionKey) => void
}

function point(cx: number, cy: number, r: number, i: number) {
  const angle = -Math.PI / 2 + (i * 2 * Math.PI) / DIMENSION_KEYS.length
  return {
    x: cx + r * Math.cos(angle),
    y: cy + r * Math.sin(angle),
  }
}

function Radar({
  levels,
  focus,
  onFocus,
}: {
  levels: Record<string, number>
  focus?: DimensionKey
  onFocus?: (key: DimensionKey) => void
}) {
  const size = 300
  const cx = size / 2
  const cy = size / 2
  const maxR = 92
  const rings = [25, 50, 75, 100]

  const valuePts = DIMENSION_KEYS.map((key, i) => {
    const level = Math.max(0, Math.min(100, levels[key] ?? 0))
    return point(cx, cy, (level / 100) * maxR, i)
  })
  const valuePath = valuePts.map((p) => `${p.x},${p.y}`).join(' ')

  return (
    <svg className="radar" viewBox={`0 0 ${size} ${size}`} role="img" aria-label="九维雷达图">
      <defs>
        <radialGradient id="radarGlow" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="var(--gold-hi)" stopOpacity="0.3" />
          <stop offset="70%" stopColor="var(--cinnabar)" stopOpacity="0.1" />
          <stop offset="100%" stopColor="transparent" stopOpacity="0" />
        </radialGradient>
      </defs>

      {/* Center ambient glow */}
      <circle cx={cx} cy={cy} r={maxR + 32} fill="url(#radarGlow)" />

      {/* Concentric grid rings */}
      {rings.map((pct) => {
        const pts = DIMENSION_KEYS.map((_, i) => point(cx, cy, (pct / 100) * maxR, i))
        return (
          <polygon
            key={pct}
            points={pts.map((p) => `${p.x},${p.y}`).join(' ')}
            className="radar-ring"
            stroke={pct === 100 ? 'rgba(247, 203, 104, 0.22)' : 'rgba(255, 255, 255, 0.08)'}
          />
        )
      })}

      {/* Dimensional axes */}
      {DIMENSION_KEYS.map((key, i) => {
        const p = point(cx, cy, maxR, i)
        const isFocus = focus === key
        return (
          <line
            key={i}
            x1={cx}
            y1={cy}
            x2={p.x}
            y2={p.y}
            className="radar-axis"
            stroke={isFocus ? 'var(--gold-hi)' : 'rgba(255, 255, 255, 0.1)'}
            strokeWidth={isFocus ? 1.5 : 1}
          />
        )
      })}

      {/* Polygon radar area */}
      <polygon points={valuePath} className="radar-value" />

      {/* Vertex indicator points */}
      {valuePts.map((p, i) => {
        const key = DIMENSION_KEYS[i]
        const isFocus = focus === key
        return (
          <g key={`pt-${key}`} style={{ cursor: 'pointer' }} onClick={() => onFocus?.(key)}>
            {isFocus && (
              <circle
                cx={p.x}
                cy={p.y}
                r={7}
                fill="none"
                stroke="var(--gold-hi)"
                strokeWidth={1.5}
                opacity={0.7}
              />
            )}
            <circle
              cx={p.x}
              cy={p.y}
              r={isFocus ? 4.5 : 3}
              fill={isFocus ? 'var(--gold-hi)' : 'var(--ink)'}
              stroke="#070a0f"
              strokeWidth={1.5}
            />
          </g>
        )
      })}

      {/* Dimension labels */}
      {DIMENSION_KEYS.map((key, i) => {
        const p = point(cx, cy, maxR + 32, i)
        const active = focus === key
        return (
          <text
            key={key}
            x={p.x}
            y={p.y}
            className="radar-label"
            textAnchor="middle"
            dominantBaseline="middle"
            fill={active ? 'var(--gold-hi)' : 'var(--ink-soft)'}
            style={{
              cursor: 'pointer',
              fontWeight: active ? 700 : 500,
              fontSize: active ? '12px' : '11px',
              transition: 'all 0.15s ease',
            }}
            onClick={() => onFocus?.(key)}
          >
            {DIMENSION_LABELS[key]}
          </text>
        )
      })}
    </svg>
  )
}

export default function DimensionSliders({
  levels,
  dimensions,
  onChange,
  disabled,
  focus,
  onFocus,
}: Props) {
  const current = focus ?? 'sentence_rhythm'
  const dim = dimensions?.[current]
  const value = levels[current] ?? 0
  const techniques = dim?.techniques ?? []

  function adjust(delta: number) {
    if (disabled || !onChange) return
    const next = Math.max(0, Math.min(100, value + delta))
    onChange(current, next)
  }

  return (
    <div className="dim-sliders">
      <div className="radar-wrap">
        <Radar levels={levels} focus={current} onFocus={onFocus} />
      </div>
      <div className="stack">
        <div className="dim-pills">
          {DIMENSION_KEYS.map((key) => (
            <button
              key={key}
              type="button"
              className={`dim-pill${current === key ? ' on' : ''}`}
              onClick={() => onFocus?.(key)}
            >
              {DIMENSION_LABELS[key]}
              <em>{levels[key] ?? 0}</em>
            </button>
          ))}
        </div>
        <div className="dim-row">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.8rem' }}>
            <span style={{ fontFamily: 'var(--font-serif)', fontSize: '1.2rem', fontWeight: 700, color: 'var(--ink)' }}>
              {DIMENSION_LABELS[current]}
            </span>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
              <button
                type="button"
                className="btn secondary sm"
                style={{ width: '28px', height: '28px', padding: 0 }}
                disabled={disabled || !onChange || value <= 0}
                onClick={() => adjust(-5)}
                title="减少 5"
              >
                <Minus size={14} />
              </button>
              <span
                style={{
                  fontFamily: 'var(--mono)',
                  fontSize: '1.3rem',
                  fontWeight: 700,
                  color: 'var(--gold-hi)',
                  minWidth: '2.5rem',
                  textAlign: 'center',
                }}
              >
                {value}
              </span>
              <button
                type="button"
                className="btn secondary sm"
                style={{ width: '28px', height: '28px', padding: 0 }}
                disabled={disabled || !onChange || value >= 100}
                onClick={() => adjust(5)}
                title="增加 5"
              >
                <Plus size={14} />
              </button>
            </div>
          </div>

          <label>
            <input
              type="range"
              min={0}
              max={100}
              step={1}
              value={value}
              disabled={disabled || !onChange}
              onChange={(e) => onChange?.(current, Number(e.target.value))}
            />
          </label>
          <p className="dim-summary">{dim?.summary ?? '当前维度处于均衡基准，尚未记录特殊技法偏好。'}</p>
          {techniques.length > 0 ? (
            <ul className="dim-techniques">
              {techniques.map((tech) => (
                <li key={tech}>{tech}</li>
              ))}
            </ul>
          ) : null}
        </div>
      </div>
    </div>
  )
}
