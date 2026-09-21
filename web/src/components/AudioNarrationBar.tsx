import { useState, useEffect, useRef } from 'react'
import { Headphones, Pause, Play, Square, X } from 'lucide-react'

interface AudioNarrationBarProps {
  title: string
  text: string
  onClose: () => void
}

export default function AudioNarrationBar({ title, text, onClose }: AudioNarrationBarProps) {
  const [playing, setPlaying] = useState(false)
  const [rate, setRate] = useState(1.1)
  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([])
  const [selectedVoice, setSelectedVoice] = useState<string>('')
  const supported = typeof window !== 'undefined' && 'speechSynthesis' in window
  const synthRef = useRef<SpeechSynthesis | null>(null)
  const utterRef = useRef<SpeechSynthesisUtterance | null>(null)

  useEffect(() => {
    if (typeof window !== 'undefined' && 'speechSynthesis' in window) {
      synthRef.current = window.speechSynthesis
      const loadVoices = () => {
        const list = synthRef.current?.getVoices() ?? []
        const zhVoices = list.filter((v) => v.lang.includes('zh') || v.lang.includes('cmn'))
        setVoices(zhVoices.length ? zhVoices : list)
        if (zhVoices.length) {
          setSelectedVoice(zhVoices[0].voiceURI)
        }
      }
      loadVoices()
      if (synthRef.current.onvoiceschanged !== undefined) {
        synthRef.current.onvoiceschanged = loadVoices
      }
    }
    return () => {
      if (synthRef.current) {
        synthRef.current.cancel()
      }
    }
  }, [])

  function handlePlay() {
    if (!synthRef.current || !text) return

    if (synthRef.current.paused) {
      synthRef.current.resume()
      setPlaying(true)
      return
    }

    synthRef.current.cancel()
    const cleanText = text.replace(/<[^>]+>/g, '').replace(/#+/g, '')
    const utter = new SpeechSynthesisUtterance(cleanText)
    utter.rate = rate
    if (selectedVoice) {
      const v = voices.find((vox) => vox.voiceURI === selectedVoice)
      if (v) utter.voice = v
    }
    utter.onend = () => setPlaying(false)
    utter.onerror = () => setPlaying(false)
    utterRef.current = utter

    synthRef.current.speak(utter)
    setPlaying(true)
  }

  // speak() with explicit overrides, used when a setting changes mid-playback.
  function restartAt(nextRate: number, nextVoice: string) {
    if (!synthRef.current || !text) return
    synthRef.current.cancel()
    const cleanText = text.replace(/<[^>]+>/g, '').replace(/#+/g, '')
    const utter = new SpeechSynthesisUtterance(cleanText)
    utter.rate = nextRate
    const v = voices.find((vox) => vox.voiceURI === nextVoice)
    if (v) utter.voice = v
    utter.onend = () => setPlaying(false)
    utter.onerror = () => setPlaying(false)
    utterRef.current = utter
    synthRef.current.speak(utter)
    setPlaying(true)
  }

  function handlePause() {
    if (!synthRef.current) return
    synthRef.current.pause()
    setPlaying(false)
  }

  function handleStop() {
    if (!synthRef.current) return
    synthRef.current.cancel()
    setPlaying(false)
  }

  return (
    <div
      style={{
        position: 'fixed',
        bottom: '20px',
        left: '50%',
        transform: 'translateX(-50%)',
        width: '620px',
        maxWidth: '92vw',
        background: 'rgba(15, 20, 32, 0.95)',
        backdropFilter: 'blur(16px)',
        borderRadius: '12px',
        border: '1px solid rgba(212, 175, 55, 0.3)',
        boxShadow: '0 16px 40px rgba(0, 0, 0, 0.7), inset 0 1px 0 rgba(255, 215, 0, 0.15)',
        zIndex: 100,
        padding: '0.8rem 1.2rem',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '1rem',
        animation: 'fadeInUp 0.3s ease-out',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem', minWidth: 0 }}>
        <div
          style={{
            width: '36px',
            height: '36px',
            borderRadius: '50%',
            background: 'rgba(212, 175, 55, 0.2)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          <Headphones size={18} color="var(--gold-hi)" />
        </div>
        <div style={{ minWidth: 0 }}>
          <strong style={{ color: '#fff', fontSize: '0.9rem', display: 'block', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {title || '章节朗读'}
          </strong>
          <span className="muted" style={{ fontSize: '0.75rem' }}>
            {!supported ? '当前浏览器不支持语音朗读' : playing ? '🔊 正在自然沉浸朗读…' : '⏸ 已暂停'}
          </span>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
        <button
          className="btn icon-only"
          type="button"
          onClick={playing ? handlePause : handlePlay}
          disabled={!supported}
          title={supported ? (playing ? '暂停' : '播放') : '当前浏览器不支持语音朗读'}
          style={{ width: '36px', height: '36px' }}
        >
          {playing ? <Pause size={16} /> : <Play size={16} />}
        </button>

        <button
          className="btn icon-only secondary"
          type="button"
          onClick={handleStop}
          title="停止"
          style={{ width: '36px', height: '36px' }}
        >
          <Square size={14} />
        </button>

        {voices.length > 1 ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.3rem', marginLeft: '0.4rem' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--ink-soft)' }}>音色</span>
            <select
              value={selectedVoice}
              onChange={(e) => {
                const next = e.target.value
                const wasPlaying = playing
                setSelectedVoice(next)
                if (wasPlaying) restartAt(rate, next)
              }}
              style={{ padding: '0.2rem 0.4rem', fontSize: '0.78rem', background: 'rgba(0,0,0,0.4)', borderRadius: '4px', color: '#fff', border: '1px solid var(--line)', maxWidth: '150px' }}
            >
              {voices.map((v) => (
                <option key={v.voiceURI} value={v.voiceURI}>
                  {v.name}
                </option>
              ))}
            </select>
          </div>
        ) : null}

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.3rem', marginLeft: '0.4rem' }}>
          <span style={{ fontSize: '0.75rem', color: 'var(--ink-soft)' }}>语速</span>
          <select
            value={rate}
            onChange={(e) => {
              const r = Number(e.target.value)
              const wasPlaying = playing
              setRate(r)
              // rate is fixed once an utterance is speaking, so restart with a
              // fresh one rather than leaving playback silently dead.
              if (wasPlaying) restartAt(r, selectedVoice)
            }}
            style={{ padding: '0.2rem 0.4rem', fontSize: '0.78rem', background: 'rgba(0,0,0,0.4)', borderRadius: '4px', color: '#fff', border: '1px solid var(--line)' }}
          >
            <option value={0.8}>0.8x</option>
            <option value={1.0}>1.0x</option>
            <option value={1.1}>1.1x</option>
            <option value={1.25}>1.25x</option>
            <option value={1.5}>1.5x</option>
          </select>
        </div>

        <button
          className="btn icon-only secondary"
          type="button"
          onClick={() => {
            handleStop()
            onClose()
          }}
          title="关闭播放器"
          style={{ width: '32px', height: '32px', marginLeft: '0.4rem' }}
        >
          <X size={16} />
        </button>
      </div>
    </div>
  )
}
