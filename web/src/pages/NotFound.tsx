import { Compass, Home, Sparkles } from 'lucide-react'
import { Link } from 'react-router-dom'
import { usePageTitle } from '../hooks'

export default function NotFound() {
  usePageTitle('404 · 虚空裂隙')
  return (
    <div className="page" style={{ minHeight: '75vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <div
        className="notfound panel"
        style={{
          maxWidth: '540px',
          width: '100%',
          textAlign: 'center',
          padding: '3.5rem 2rem',
          position: 'relative',
          overflow: 'hidden',
          background: 'linear-gradient(180deg, rgba(17, 24, 38, 0.9) 0%, rgba(9, 12, 18, 0.95) 100%)',
          borderColor: 'rgba(212, 175, 55, 0.25)',
          boxShadow: '0 24px 60px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 215, 0, 0.1)',
        }}
      >
        <div
          style={{
            width: '80px',
            height: '80px',
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(224, 104, 79, 0.2) 0%, transparent 70%)',
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginBottom: '1.5rem',
            border: '1px solid rgba(224, 104, 79, 0.3)',
          }}
        >
          <Compass size={40} color="var(--gold-hi)" style={{ animation: 'spin 20s linear infinite' }} />
        </div>

        <p
          style={{
            fontFamily: 'Cinzel, serif',
            fontSize: '3.5rem',
            fontWeight: 900,
            letterSpacing: '0.2em',
            lineHeight: 1,
            margin: '0 0 1rem',
            background: 'linear-gradient(135deg, #ffd700 0%, #e0684f 100%)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
          }}
        >
          404
        </p>

        <h1 style={{ fontSize: '1.4rem', margin: '0 0 0.8rem', color: '#fff' }}>虚空裂隙 · 页面迷失</h1>
        <p className="muted" style={{ maxWidth: '380px', margin: '0 auto 2rem', fontSize: '0.92rem', lineHeight: 1.6 }}>
          此方天地尚未开辟，或是时空坐标已被重构。请返回作品母体继续撰写宏篇。
        </p>

        <div style={{ display: 'flex', gap: '0.8rem', justifyContent: 'center', flexWrap: 'wrap' }}>
          <Link className="btn" to="/" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}>
            <Home size={16} />
            回作品集
          </Link>
          <Link className="btn secondary" to="/settings" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}>
            <Sparkles size={16} />
            模型配置
          </Link>
        </div>
      </div>
    </div>
  )
}
