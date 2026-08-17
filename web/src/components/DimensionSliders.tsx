import { DIMENSION_KEYS, DIMENSION_LABELS, type Dimension, type DimensionKey } from '../types'

type Props = {
  levels: Record<string, number>
  dimensions?: Record<string, Dimension>
  onChange?: (key: DimensionKey, value: number) => void
  disabled?: boolean
}

function point(cx: number, cy: number, r: number, i: number) {
  const angle = (-Math.PI / 2 + (i * 2 * Math.PI) / DIMENSION_KEYS.length)
  return {
    x: cx + r * Math.cos(angle),
    y: cy + r * Math.sin(angle),
  }
}

function Radar({ levels }: { levels: Record<string, number> }) {
  const size = 280
  const cx = size / 2
  const cy = size / 2
  const maxR = 88
  const rings = [25, 50, 75, 100]

  const valuePts = DIMENSION_KEYS.map((key, i) => {
    const level = Math.max(0, Math.min(100, levels[key] ?? 0))
    return point(cx, cy, (level / 100) * maxR, i)
  })
  const valuePath = valuePts.map((p) => `${p.x},${p.y}`).join(' ')

  return (
    <svg className="radar" viewBox={`0 0 ${size} ${size}`} role="img" aria-label="九维雷达">
      {rings.map((pct) => {
        const pts = DIMENSION_KEYS.map((_, i) => point(cx, cy, (pct / 100) * maxR, i))
        return (
          <polygon
            key={pct}
            points={pts.map((p) => `${p.x},${p.y}`).join(' ')}
            className="radar-ring"
          />
        )
      })}
      {DIMENSION_KEYS.map((_, i) => {
        const p = point(cx, cy, maxR, i)
        return <line key={i} x1={cx} y1={cy} x2={p.x} y2={p.y} className="radar-axis" />
      })}
      <polygon points={valuePath} className="radar-value" />
      {DIMENSION_KEYS.map((key, i) => {
        const p = point(cx, cy, maxR + 28, i)
        return (
          <text key={key} x={p.x} y={p.y} className="radar-label" textAnchor="middle" dominantBaseline="middle">
            {DIMENSION_LABELS[key]}
          </text>
        )
      })}
    </svg>
  )
}

export default function DimensionSliders({ levels, dimensions, onChange, disabled }: Props) {
  return (
    <div className="dim-sliders">
      <Radar levels={levels} />
      <div className="stack">
        {DIMENSION_KEYS.map((key) => {
          const value = levels[key] ?? 0
          const dim = dimensions?.[key]
          const techniques = dim?.techniques ?? []
          return (
            <div key={key} className="dim-row">
              <label>
                <span>
                  {DIMENSION_LABELS[key]}
                  <span className="muted"> {value}</span>
                </span>
                <input
                  type="range"
                  min={0}
                  max={100}
                  step={1}
                  value={value}
                  disabled={disabled || !onChange}
                  onChange={(e) => onChange?.(key, Number(e.target.value))}
                />
              </label>
              <p className="dim-summary muted">{dim?.summary ?? ''}</p>
              {techniques.length > 0 ? (
                <ul className="dim-techniques">
                  {techniques.map((tech) => (
                    <li key={tech}>{tech}</li>
                  ))}
                </ul>
              ) : null}
            </div>
          )
        })}
      </div>
    </div>
  )
}
