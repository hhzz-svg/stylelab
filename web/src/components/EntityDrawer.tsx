import { useState } from 'react'
import {
  Link2,
  Plus,
  Save,
  Trash2,
  X,
  Zap,
} from 'lucide-react'
import { APIError, api, notify } from '../api'
import { confirm } from './ConfirmDialog'
import type { GraphEdge, GraphNode, GraphNodeKind } from '../types'

const KIND_MAP: Record<GraphNodeKind, { label: string; icon: string; color: string }> = {
  character: { label: '人物角色', icon: '🧑', color: 'var(--gold-hi)' },
  faction: { label: '门派势力', icon: '🏯', color: 'var(--jade-hi)' },
  artifact: { label: '法宝神器', icon: '🔮', color: 'var(--violet-hi)' },
  location: { label: '地理秘境', icon: '🗺️', color: 'var(--cinnabar-hi)' },
}

type Props = {
  projectId: string
  node: GraphNode | null
  allNodes: GraphNode[]
  edges: GraphEdge[]
  open: boolean
  onClose: () => void
  onNodeUpdated: () => void
  onEdgeUpdated: () => void
}

export default function EntityDrawer({
  projectId,
  node,
  allNodes,
  edges,
  open,
  onClose,
  onNodeUpdated,
  onEdgeUpdated,
}: Props) {
  if (!node || !open) return null

  const [name, setName] = useState(node.name)
  const [kind, setKind] = useState<GraphNodeKind>(node.kind)
  const [faction, setFaction] = useState(node.faction || '')
  const [summary, setSummary] = useState(node.summary || '')
  const [realm, setRealm] = useState<string>(node.details?.realm || '')
  const [temperament, setTemperament] = useState<string>(node.details?.temperament || '')
  const [secrets, setSecrets] = useState<string>(node.details?.secrets || '')
  const [saving, setSaving] = useState(false)

  // Add edge state
  const [showAddEdge, setShowAddEdge] = useState(false)
  const [targetId, setTargetId] = useState('')
  const [relationName, setRelationName] = useState('')
  const [relationDesc, setRelationDesc] = useState('')
  const [savingEdge, setSavingEdge] = useState(false)

  // Find all edges connected to this node
  const incidentEdges = edges.filter(
    (e) => e.source_id === node.id || e.target_id === node.id,
  )

  const otherNodes = allNodes.filter((n) => n.id !== node.id)

  async function handleSaveNode(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    try {
      await api.saveGraphNode(projectId, {
        id: node!.id,
        name,
        kind,
        faction,
        summary,
        details: {
          ...node!.details,
          realm,
          temperament,
          secrets,
        },
      })
      notify(`已保存「${name}」档案`, 'success')
      onNodeUpdated()
    } catch (err) {
      notify(err instanceof APIError ? err.message : '保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  async function handleDeleteNode() {
    const ok = await confirm({
      title: `删除实体「${node!.name}」？`,
      body: '该实体的档案及其所有关联关系线将被彻底移除。',
      confirmText: '确认删除',
      danger: true,
    })
    if (!ok) return

    try {
      await api.deleteGraphNode(projectId, node!.id)
      notify(`已删除实体「${node!.name}」`, 'success')
      onClose()
      onNodeUpdated()
    } catch (err) {
      notify(err instanceof APIError ? err.message : '删除失败', 'error')
    }
  }

  async function handleCreateEdge(e: React.FormEvent) {
    e.preventDefault()
    if (!targetId || !relationName.trim()) {
      notify('请选择目标对象并填写关系名称', 'error')
      return
    }
    setSavingEdge(true)
    try {
      await api.saveGraphEdge(projectId, {
        source_id: node!.id,
        target_id: targetId,
        relation: relationName.trim(),
        description: relationDesc.trim(),
        strength: 1.0,
      })
      notify('已建立关联关系', 'success')
      setShowAddEdge(false)
      setTargetId('')
      setRelationName('')
      setRelationDesc('')
      onEdgeUpdated()
    } catch (err) {
      notify(err instanceof APIError ? err.message : '添加关系失败', 'error')
    } finally {
      setSavingEdge(false)
    }
  }

  async function handleDeleteEdge(edgeId: string) {
    try {
      await api.deleteGraphEdge(projectId, edgeId)
      notify('已移除该关系', 'success')
      onEdgeUpdated()
    } catch (err) {
      notify(err instanceof APIError ? err.message : '删除关系失败', 'error')
    }
  }

  const kindMeta = KIND_MAP[kind] ?? KIND_MAP.character

  return (
    <div className="entity-drawer-wrap" onClick={onClose}>
      <div className="entity-drawer" onClick={(e) => e.stopPropagation()}>
        <div className="entity-drawer-head">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.7rem' }}>
            <span className="entity-kind-avatar" style={{ background: `${kindMeta.color}15`, borderColor: kindMeta.color }}>
              {kindMeta.icon}
            </span>
            <div>
              <h2 className="entity-drawer-title">{node.name}</h2>
              <span className="entity-drawer-sub">
                {kindMeta.label} {faction ? `· ${faction}` : ''}
              </span>
            </div>
          </div>
          <button className="icon-btn" type="button" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="entity-drawer-body">
          {/* 基本信息编辑表单 */}
          <form className="stack" onSubmit={handleSaveNode}>
            <div className="two-col">
              <label>
                名称
                <input value={name} onChange={(e) => setName(e.target.value)} required />
              </label>
              <label>
                类型
                <select value={kind} onChange={(e) => setKind(e.target.value as GraphNodeKind)}>
                  {Object.entries(KIND_MAP).map(([k, v]) => (
                    <option key={k} value={k}>
                      {v.icon} {v.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <label>
              所属势力 / 门派阵营
              <input
                value={faction}
                onChange={(e) => setFaction(e.target.value)}
                placeholder="例如：落云宗、魔道六宗、天南皇室"
              />
            </label>

            <label>
              核心定位 / 一句话总结
              <textarea
                value={summary}
                onChange={(e) => setSummary(e.target.value)}
                rows={2}
                placeholder="实体核心身份、特征与登场定位"
              />
            </label>

            {kind === 'character' && (
              <>
                <div className="two-col">
                  <label>
                    当前境界 / 修为等级
                    <input
                      value={realm}
                      onChange={(e) => setRealm(e.target.value)}
                      placeholder="例如：结丹后期、化神初期"
                    />
                  </label>
                  <label>
                    性格脾气 / 行事准则
                    <input
                      value={temperament}
                      onChange={(e) => setTemperament(e.target.value)}
                      placeholder="例如：谨小慎微、杀伐果断、重诺守信"
                    />
                  </label>
                </div>

                <label>
                  身怀秘密 / 动机与底牌
                  <textarea
                    value={secrets}
                    onChange={(e) => setSecrets(e.target.value)}
                    rows={2}
                    placeholder="不为人知的隐藏身份、随身法宝或执念动机"
                  />
                </label>
              </>
            )}

            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '0.4rem' }}>
              <button
                type="button"
                className="btn ghost sm"
                style={{ color: 'var(--cinnabar-hi)' }}
                onClick={() => void handleDeleteNode()}
              >
                <Trash2 size={14} /> 删除实体
              </button>
              <button className="btn sm" type="submit" disabled={saving || !name.trim()}>
                <Save size={14} /> {saving ? '保存中…' : '保存档案'}
              </button>
            </div>
          </form>

          {/* 关联人脉关系网络 */}
          <div className="entity-relations-section">
            <div className="entity-relations-head">
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                <Link2 size={16} color="var(--gold-hi)" />
                <h3 style={{ margin: 0, fontSize: '0.95rem', color: 'var(--ink)' }}>关联人脉与羁绊 ({incidentEdges.length})</h3>
              </div>
              <button
                className="btn secondary sm"
                type="button"
                onClick={() => setShowAddEdge((v) => !v)}
              >
                <Plus size={13} /> {showAddEdge ? '取消' : '添加关系'}
              </button>
            </div>

            {showAddEdge && (
              <form className="add-edge-form stack" onSubmit={handleCreateEdge}>
                <label>
                  关联目标实体
                  <select value={targetId} onChange={(e) => setTargetId(e.target.value)} required>
                    <option value="">-- 请选择目标对象 --</option>
                    {otherNodes.map((n) => (
                      <option key={n.id} value={n.id}>
                        {KIND_MAP[n.kind]?.icon} {n.name} ({n.faction || '无阵营'})
                      </option>
                    ))}
                  </select>
                </label>

                <div className="two-col">
                  <label>
                    关系类型
                    <input
                      value={relationName}
                      onChange={(e) => setRelationName(e.target.value)}
                      placeholder="如：盟友 / 宿敌 / 师徒 / 暗恋 / 主仆"
                      required
                    />
                  </label>
                  <label>
                    关系渊源或冲突点（选填）
                    <input
                      value={relationDesc}
                      onChange={(e) => setRelationDesc(e.target.value)}
                      placeholder="如：因争夺古修遗迹结怨"
                    />
                  </label>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <button className="btn sm" type="submit" disabled={savingEdge || !targetId || !relationName.trim()}>
                    <Zap size={14} /> {savingEdge ? '正在建立…' : '确认建立关系'}
                  </button>
                </div>
              </form>
            )}

            <div className="incident-edges-list">
              {incidentEdges.length === 0 ? (
                <p className="muted" style={{ fontSize: '0.82rem', padding: '0.6rem 0' }}>
                  暂未建立任何关联关系。可点击「添加关系」或使用 AI 一键扫描全书关系网。
                </p>
              ) : (
                incidentEdges.map((edge) => {
                  const isSource = edge.source_id === node.id
                  const otherId = isSource ? edge.target_id : edge.source_id
                  const otherNode = allNodes.find((n) => n.id === otherId)
                  return (
                    <div key={edge.id} className="incident-edge-item">
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
                        <span className="relation-tag">{edge.relation}</span>
                        <span style={{ fontWeight: 600, color: 'var(--ink)' }}>
                          {isSource ? '➡️ ' : '⬅️ '}
                          {otherNode?.name ?? '未知对象'}
                        </span>
                        {edge.description && (
                          <span className="relation-desc muted">({edge.description})</span>
                        )}
                      </div>
                      <button
                        className="icon-btn sm"
                        type="button"
                        onClick={() => void handleDeleteEdge(edge.id)}
                        title="移除关系"
                      >
                        <Trash2 size={13} color="var(--cinnabar-hi)" />
                      </button>
                    </div>
                  )
                })
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
