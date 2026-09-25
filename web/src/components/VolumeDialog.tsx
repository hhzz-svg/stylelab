import { type FormEvent, useState } from 'react'
import { api, errMessage } from '../api'
import type { Volume } from '../types'

type Props = {
  projectId: string
  /** Edit this volume, or create one starting at startSeq. */
  volume?: Volume
  startSeq?: number
  onClose: () => void
  onSaved: () => void
}

/** 新建 / 编辑一卷：卷从某一章开始，直到下一卷之前。 */
export default function VolumeDialog({ projectId, volume, startSeq, onClose, onSaved }: Props) {
  const [title, setTitle] = useState(volume?.title ?? '')
  const [brief, setBrief] = useState(volume?.brief ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const start = volume?.start_seq ?? startSeq ?? 1

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      if (volume) {
        await api.updateVolume(volume.id, { title, brief })
      } else {
        await api.createVolume(projectId, { start_seq: start, title, brief })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(errMessage(err, '保存分卷失败'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal-panel"
        role="dialog"
        aria-modal="true"
        aria-label={volume ? '编辑分卷' : '新建分卷'}
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: '460px' }}
      >
        <h2 style={{ fontFamily: 'var(--font-serif)', fontSize: '1.2rem', marginTop: 0 }}>
          {volume ? '编辑分卷' : `从第 ${start} 章开始新的一卷`}
        </h2>
        <form className="stack" onSubmit={submit}>
          <label>
            卷名
            <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="如：第一卷 · 少年出山" required autoFocus />
          </label>
          <label>
            本卷简介
            <textarea value={brief} onChange={(e) => setBrief(e.target.value)} rows={3} placeholder="本卷主线目标与高潮收尾（可选）" />
          </label>
          {error ? <p className="error">{error}</p> : null}
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.6rem' }}>
            <button className="btn secondary sm" type="button" onClick={onClose}>
              取消
            </button>
            <button className="btn sm" type="submit" disabled={saving || !title.trim()}>
              {saving ? '保存中…' : '保存'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
