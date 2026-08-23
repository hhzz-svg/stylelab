import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Network,
  Plus,
  RotateCcw,
  Search,
  Sparkles,
  UserPlus,
  ZoomIn,
  ZoomOut,
} from 'lucide-react'
import { useParams } from 'react-router-dom'
import { APIError, api, notify } from '../api'
import Crumb from '../components/Crumb'
import EntityDrawer from '../components/EntityDrawer'
import Skeleton from '../components/Skeleton'
import { usePageTitle } from '../hooks'
import type { GraphData, GraphNode, GraphNodeKind } from '../types'

// 颜色映射池
const FACTION_COLORS = [
  '#f7cb68', // 金
  '#2dd4bf', // 碧
  '#a78bfa', // 紫
  '#f87171', // 赤
  '#38bdf8', // 蓝
  '#fb923c', // 橙
  '#4ade80', // 翠
  '#f472b6', // 粉
]

function getFactionColor(faction: string): string {
  if (!faction || faction === '无阵营' || faction === '散修') return '#94a3b8'
  let hash = 0
  for (let i = 0; i < faction.length; i++) {
    hash = faction.charCodeAt(i) + ((hash << 5) - hash)
  }
  return FACTION_COLORS[Math.abs(hash) % FACTION_COLORS.length]
}

const KIND_ICONS: Record<GraphNodeKind, string> = {
  character: '🧑',
  faction: '🏯',
  artifact: '🔮',
  location: '🗺️',
}

interface SimNode extends GraphNode {
  x: number
  y: number
  vx: number
  vy: number
  fx?: number | null
  fy?: number | null
}

export default function Graph() {
  const { id } = useParams()
  const projectId = id ?? ''
  usePageTitle('人物与势力关系图谱')

  const [graphData, setGraphData] = useState<GraphData | null>(null)
  const [loading, setLoading] = useState(true)
  const [extracting, setExtracting] = useState(false)
  const [error, setError] = useState('')

  // Filter & Search
  const [searchQuery, setSearchQuery] = useState('')
  const [activeKind, setActiveKind] = useState<string>('all')

  // Selected node for drawer
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)

  // Add node modal
  const [showAddNodeModal, setShowAddNodeModal] = useState(false)
  const [newNodeName, setNewNodeName] = useState('')
  const [newNodeKind, setNewNodeKind] = useState<GraphNodeKind>('character')
  const [newNodeFaction, setNewNodeFaction] = useState('')
  const [newNodeSummary, setNewNodeSummary] = useState('')
  const [creatingNode, setCreatingNode] = useState(false)

  // Canvas zoom & pan
  const [zoom, setZoom] = useState(1.0)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [isPanning, setIsPanning] = useState(false)
  const [panStart, setPanStart] = useState({ x: 0, y: 0 })

  // Hovered node for 1-hop highlighting
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null)

  // Simulation ref
  const simNodesRef = useRef<SimNode[]>([])
  const animationFrameRef = useRef<number>(0)
  const draggingNodeRef = useRef<SimNode | null>(null)
  const [, setTick] = useState(0)

  async function loadGraph() {
    if (!projectId) return
    try {
      const data = await api.getGraph(projectId)
      setGraphData(data)
      initSimulation(data.nodes)
      setError('')
    } catch (err) {
      setError(err instanceof APIError ? err.message : '加载关系图谱失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!projectId) return
    void loadGraph()
  }, [projectId])

  // Initialize physical nodes in circular layout
  function initSimulation(nodes: GraphNode[]) {
    const width = 1000
    const height = 700
    const centerX = width / 2
    const centerY = height / 2
    const radius = Math.min(centerX, centerY) * 0.65

    const simNodes: SimNode[] = nodes.map((n, idx) => {
      const angle = (idx / Math.max(1, nodes.length)) * 2 * Math.PI
      const x = n.x !== 0 ? n.x : centerX + radius * Math.cos(angle) + (Math.random() - 0.5) * 40
      const y = n.y !== 0 ? n.y : centerY + radius * Math.sin(angle) + (Math.random() - 0.5) * 40
      return {
        ...n,
        x,
        y,
        vx: (Math.random() - 0.5) * 2,
        vy: (Math.random() - 0.5) * 2,
      }
    })

    simNodesRef.current = simNodes
    startSimulation()
  }

  // Force simulation loop
  function startSimulation() {
    cancelAnimationFrame(animationFrameRef.current)

    const width = 1000
    const height = 700
    const centerX = width / 2
    const centerY = height / 2

    let iterations = 0
    const maxIterations = 350

    function step() {
      const nodes = simNodesRef.current
      const edges = graphData?.edges ?? []

      if (!nodes || nodes.length === 0) return

      // 1. Repulsion between all pairs (Coulomb)
      const kRep = 3500
      for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
          const n1 = nodes[i]
          const n2 = nodes[j]
          const dx = n2.x - n1.x
          const dy = n2.y - n1.y
          const distSq = dx * dx + dy * dy + 100
          const dist = Math.sqrt(distSq)
          const force = kRep / distSq

          const fx = (dx / dist) * force
          const fy = (dy / dist) * force

          if (!n1.fx) {
            n1.vx -= fx
            n1.vy -= fy
          }
          if (!n2.fx) {
            n2.vx += fx
            n2.vy += fy
          }
        }
      }

      // 2. Spring attraction along edges (Hooke's law)
      const nodeMap = new Map<string, SimNode>()
      nodes.forEach((n) => nodeMap.set(n.id, n))

      const kSpring = 0.04
      const targetDist = 140

      for (const edge of edges) {
        const src = nodeMap.get(edge.source_id)
        const tgt = nodeMap.get(edge.target_id)
        if (!src || !tgt) continue

        const dx = tgt.x - src.x
        const dy = tgt.y - src.y
        const dist = Math.sqrt(dx * dx + dy * dy) || 1
        const delta = dist - targetDist
        const force = kSpring * delta

        const fx = (dx / dist) * force
        const fy = (dy / dist) * force

        if (!src.fx) {
          src.vx += fx
          src.vy += fy
        }
        if (!tgt.fx) {
          tgt.vx -= fx
          tgt.vy -= fy
        }
      }

      // 3. Center gravity pull & velocity damping
      const kCenter = 0.008
      const damping = 0.88

      let totalEnergy = 0

      for (const n of nodes) {
        if (n.fx != null && n.fy != null) {
          n.x = n.fx
          n.y = n.fy
          n.vx = 0
          n.vy = 0
        } else {
          n.vx += (centerX - n.x) * kCenter
          n.vy += (centerY - n.y) * kCenter
          n.vx *= damping
          n.vy *= damping
          n.x += n.vx
          n.y += n.vy
        }
        totalEnergy += Math.abs(n.vx) + Math.abs(n.vy)
      }

      iterations++
      setTick((t) => (t + 1) % 1000000)

      if (iterations < maxIterations || draggingNodeRef.current || totalEnergy > 0.5) {
        animationFrameRef.current = requestAnimationFrame(step)
      }
    }

    animationFrameRef.current = requestAnimationFrame(step)
  }

  // Handle node drag
  function handleNodePointerDown(e: React.PointerEvent, node: SimNode) {
    e.stopPropagation()
    ;(e.target as Element).setPointerCapture(e.pointerId)
    draggingNodeRef.current = node
    node.fx = node.x
    node.fy = node.y
    startSimulation()
  }

  function handleNodePointerMove(e: React.PointerEvent) {
    if (!draggingNodeRef.current) return
    const svg = e.currentTarget.closest('svg')
    if (!svg) return
    const rect = svg.getBoundingClientRect()
    // Convert screen coordinates to SVG transformed coordinate space
    const mouseX = (e.clientX - rect.left - pan.x) / zoom
    const mouseY = (e.clientY - rect.top - pan.y) / zoom

    draggingNodeRef.current.fx = mouseX
    draggingNodeRef.current.fy = mouseY
  }

  // Handle canvas pan & zoom
  function handleCanvasPointerDown(e: React.PointerEvent) {
    if (e.button !== 0) return // Only primary button
    setIsPanning(true)
    setPanStart({ x: e.clientX - pan.x, y: e.clientY - pan.y })
  }

  function handleCanvasPointerMove(e: React.PointerEvent) {
    if (draggingNodeRef.current) {
      handleNodePointerMove(e)
      return
    }
    if (!isPanning) return
    setPan({
      x: e.clientX - panStart.x,
      y: e.clientY - panStart.y,
    })
  }

  function handleCanvasPointerUp() {
    setIsPanning(false)
    if (draggingNodeRef.current) {
      draggingNodeRef.current.fx = null
      draggingNodeRef.current.fy = null
      draggingNodeRef.current = null
    }
  }

  function handleWheel(e: React.WheelEvent) {
    e.preventDefault()
    const zoomFactor = e.deltaY < 0 ? 1.1 : 0.9
    setZoom((z) => Math.min(3.0, Math.max(0.35, z * zoomFactor)))
  }

  function resetView() {
    setZoom(1.0)
    setPan({ x: 0, y: 0 })
    startSimulation()
  }

  // AI Extraction handler
  async function handleAIExtract() {
    setExtracting(true)
    try {
      notify('正在召唤 AI 深度扫描全书章节与世界书实体关系…', 'info')
      const data = await api.extractGraph(projectId)
      setGraphData(data)
      initSimulation(data.nodes)
      notify(`AI 提炼成功！已收录 ${data.nodes.length} 个实体与 ${data.edges.length} 条关系`, 'success')
    } catch (err) {
      notify(err instanceof APIError ? err.message : 'AI 扫描提炼失败', 'error')
    } finally {
      setExtracting(false)
    }
  }

  // Manual create node handler
  async function handleCreateManualNode(e: React.FormEvent) {
    e.preventDefault()
    if (!newNodeName.trim()) return
    setCreatingNode(true)
    try {
      await api.saveGraphNode(projectId, {
        name: newNodeName.trim(),
        kind: newNodeKind,
        faction: newNodeFaction.trim(),
        summary: newNodeSummary.trim(),
        x: 500 + (Math.random() - 0.5) * 80,
        y: 350 + (Math.random() - 0.5) * 80,
      })
      notify(`已创建实体「${newNodeName}」`, 'success')
      setShowAddNodeModal(false)
      setNewNodeName('')
      setNewNodeFaction('')
      setNewNodeSummary('')
      await loadGraph()
    } catch (err) {
      notify(err instanceof APIError ? err.message : '创建失败', 'error')
    } finally {
      setCreatingNode(false)
    }
  }

  // Active connected neighbors map
  const connectedNodeIds = useMemo(() => {
    if (!hoveredNodeId || !graphData) return null
    const set = new Set<string>([hoveredNodeId])
    for (const e of graphData.edges) {
      if (e.source_id === hoveredNodeId) set.add(e.target_id)
      if (e.target_id === hoveredNodeId) set.add(e.source_id)
    }
    return set
  }, [hoveredNodeId, graphData])

  // Filtered nodes
  const nodes = simNodesRef.current
  const filteredNodes = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    return nodes.filter((n) => {
      const matchesKind = activeKind === 'all' || n.kind === activeKind
      const matchesQuery = !q || n.name.toLowerCase().includes(q) || (n.faction && n.faction.toLowerCase().includes(q))
      return matchesKind && matchesQuery
    })
  }, [nodes, activeKind, searchQuery])

  const filteredNodeIdSet = useMemo(() => new Set(filteredNodes.map((n) => n.id)), [filteredNodes])

  // Edges to render
  const edges = graphData?.edges ?? []
  const nodeMap = useMemo(() => {
    const map = new Map<string, SimNode>()
    nodes.forEach((n) => map.set(n.id, n))
    return map
  }, [nodes])

  return (
    <div className="page page-wide" style={{ height: 'calc(100vh - 64px)', display: 'flex', flexDirection: 'column' }}>
      <Crumb projectId={projectId} current="关系图谱" />
      <div className="page-head" style={{ marginBottom: '0.8rem' }}>
        <div>
          <p className="kicker">LORE & CHARACTER GRAPH</p>
          <h1 style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <Network size={26} color="var(--gold-hi)" />
            人物关系与势力图谱
          </h1>
          <p className="sub">
            可视化探索小说全景人物脉络、阵营归属与因果羁绊。支持力导向物理拖拽与 AI 全书自动提炼。
          </p>
        </div>

        <div className="status-grid">
          <button
            className="btn"
            type="button"
            disabled={extracting}
            onClick={() => void handleAIExtract()}
            style={{ background: 'linear-gradient(135deg, #f7cb68 0%, #e2b04a 100%)', color: '#070a0f', fontWeight: 700 }}
          >
            {extracting ? <div className="spinner sm" /> : <Sparkles size={16} />}
            {extracting ? 'AI 正在分析全书…' : '✨ AI 从正文提炼图谱'}
          </button>
          <button
            className="btn secondary"
            type="button"
            onClick={() => setShowAddNodeModal(true)}
          >
            <UserPlus size={16} /> 添加实体
          </button>
          <div className="status-chip">
            <span>实体</span>
            <strong>{graphData?.nodes.length ?? 0}</strong>
          </div>
          <div className="status-chip">
            <span>羁绊</span>
            <strong>{graphData?.edges.length ?? 0}</strong>
          </div>
        </div>
      </div>

      {error ? <p className="error">{error}</p> : null}

      {/* 图谱控制工具栏 */}
      <div className="graph-toolbar">
        <div className="graph-filter-tabs">
          <button
            type="button"
            className={'filter-tab' + (activeKind === 'all' ? ' active' : '')}
            onClick={() => setActiveKind('all')}
          >
            全部 ({graphData?.nodes.length ?? 0})
          </button>
          <button
            type="button"
            className={'filter-tab' + (activeKind === 'character' ? ' active' : '')}
            onClick={() => setActiveKind('character')}
          >
            🧑 人物 ({graphData?.nodes.filter((n) => n.kind === 'character').length ?? 0})
          </button>
          <button
            type="button"
            className={'filter-tab' + (activeKind === 'faction' ? ' active' : '')}
            onClick={() => setActiveKind('faction')}
          >
            🏯 门派势力 ({graphData?.nodes.filter((n) => n.kind === 'faction').length ?? 0})
          </button>
          <button
            type="button"
            className={'filter-tab' + (activeKind === 'artifact' ? ' active' : '')}
            onClick={() => setActiveKind('artifact')}
          >
            🔮 法宝神器 ({graphData?.nodes.filter((n) => n.kind === 'artifact').length ?? 0})
          </button>
          <button
            type="button"
            className={'filter-tab' + (activeKind === 'location' ? ' active' : '')}
            onClick={() => setActiveKind('location')}
          >
            🗺️ 地理秘境 ({graphData?.nodes.filter((n) => n.kind === 'location').length ?? 0})
          </button>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.8rem' }}>
          <label className="search-field" style={{ minWidth: '220px', margin: 0 }}>
            <Search size={15} aria-hidden="true" />
            <input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="搜索角色名或门派…"
            />
          </label>

          <div className="graph-view-controls">
            <button className="icon-btn sm" type="button" onClick={() => setZoom((z) => Math.min(3.0, z * 1.15))} title="放大">
              <ZoomIn size={15} />
            </button>
            <button className="icon-btn sm" type="button" onClick={() => setZoom((z) => Math.max(0.35, z * 0.85))} title="缩小">
              <ZoomOut size={15} />
            </button>
            <button className="icon-btn sm" type="button" onClick={resetView} title="重置视角">
              <RotateCcw size={15} />
            </button>
          </div>
        </div>
      </div>

      {/* SVG 力导向图谱主画布 */}
      <div
        className="graph-canvas-container"
        onPointerDown={handleCanvasPointerDown}
        onPointerMove={handleCanvasPointerMove}
        onPointerUp={handleCanvasPointerUp}
        onWheel={handleWheel}
      >
        {loading ? (
          <div style={{ width: '100%', height: '100%', display: 'grid', placeItems: 'center' }}>
            <Skeleton h={400} />
          </div>
        ) : nodes.length === 0 ? (
          <div className="empty-state">
            <Network size={36} color="var(--gold-hi)" />
            <h2>关系图谱暂无实体</h2>
            <p className="muted">您可以一键让 AI 扫描现有章节与设定提炼图谱，也可以手动添加人物实体。</p>
            <div style={{ display: 'flex', gap: '0.8rem', marginTop: '1rem' }}>
              <button
                className="btn"
                type="button"
                disabled={extracting}
                onClick={() => void handleAIExtract()}
              >
                <Sparkles size={16} /> 一键 AI 提炼全书图谱
              </button>
              <button
                className="btn secondary"
                type="button"
                onClick={() => setShowAddNodeModal(true)}
              >
                <Plus size={16} /> 手动添加实体
              </button>
            </div>
          </div>
        ) : (
          <svg className="graph-svg" width="100%" height="100%">
            <defs>
              <marker
                id="arrowhead"
                viewBox="0 0 10 10"
                refX="26"
                refY="5"
                markerWidth="6"
                markerHeight="6"
                orient="auto"
              >
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="rgba(247, 203, 104, 0.6)" />
              </marker>
              <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
                <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur" />
                <feMerge>
                  <feMergeNode in="blur" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
            </defs>

            <g transform={`translate(${pan.x}, ${pan.y}) scale(${zoom})`}>
              {/* 关系连线 Edges */}
              <g className="graph-edges-layer">
                {edges.map((edge) => {
                  const src = nodeMap.get(edge.source_id)
                  const tgt = nodeMap.get(edge.target_id)
                  if (!src || !tgt) return null

                  const isConnected =
                    !hoveredNodeId ||
                    edge.source_id === hoveredNodeId ||
                    edge.target_id === hoveredNodeId

                  const isVisible =
                    filteredNodeIdSet.has(edge.source_id) &&
                    filteredNodeIdSet.has(edge.target_id)

                  if (!isVisible) return null

                  const midX = (src.x + tgt.x) / 2
                  const midY = (src.y + tgt.y) / 2

                  return (
                    <g key={edge.id} className={'graph-edge-group' + (isConnected ? ' active' : ' dimmed')}>
                      <line
                        x1={src.x}
                        y1={src.y}
                        x2={tgt.x}
                        y2={tgt.y}
                        className="graph-edge-line"
                        markerEnd="url(#arrowhead)"
                      />
                      <g transform={`translate(${midX}, ${midY})`}>
                        <rect
                          x={-edge.relation.length * 5.5 - 6}
                          y={-9}
                          width={edge.relation.length * 11 + 12}
                          height={18}
                          rx={9}
                          className="graph-edge-badge-bg"
                        />
                        <text className="graph-edge-text" textAnchor="middle" dy="4">
                          {edge.relation}
                        </text>
                      </g>
                    </g>
                  )
                })}
              </g>

              {/* 实体节点 Nodes */}
              <g className="graph-nodes-layer">
                {nodes.map((node) => {
                  const isVisible = filteredNodeIdSet.has(node.id)
                  if (!isVisible) return null

                  const isHovered = hoveredNodeId === node.id
                  const isConnected = !connectedNodeIds || connectedNodeIds.has(node.id)
                  const color = getFactionColor(node.faction)
                  const icon = KIND_ICONS[node.kind] ?? '🧑'

                  return (
                    <g
                      key={node.id}
                      className={
                        'graph-node-group' +
                        (isHovered ? ' hovered' : '') +
                        (!isConnected ? ' dimmed' : '') +
                        (selectedNode?.id === node.id ? ' selected' : '')
                      }
                      transform={`translate(${node.x}, ${node.y})`}
                      onPointerDown={(e) => handleNodePointerDown(e, node)}
                      onMouseEnter={() => setHoveredNodeId(node.id)}
                      onMouseLeave={() => setHoveredNodeId(null)}
                      onClick={(e) => {
                        e.stopPropagation()
                        setSelectedNode(node)
                        setDrawerOpen(true)
                      }}
                    >
                      {/* Faction glow ring */}
                      <circle
                        r={24}
                        fill={color}
                        fillOpacity={isHovered ? 0.25 : 0.12}
                        stroke={color}
                        strokeWidth={isHovered ? 2.5 : 1.5}
                        strokeOpacity={isHovered ? 1 : 0.6}
                        filter="url(#glow)"
                        className="node-glow-ring"
                      />

                      {/* Main node icon circle */}
                      <circle
                        r={18}
                        fill="rgba(14, 20, 31, 0.95)"
                        stroke={color}
                        strokeWidth={1.5}
                        className="node-core-circle"
                      />

                      {/* Kind Emoji */}
                      <text className="node-emoji" textAnchor="middle" dy="5" fontSize="13">
                        {icon}
                      </text>

                      {/* Node Name Label */}
                      <text
                        className="node-name-label"
                        textAnchor="middle"
                        dy="36"
                        fill={isHovered ? 'var(--gold-hi)' : '#f1f5f9'}
                      >
                        {node.name}
                      </text>

                      {/* Faction Tag */}
                      {node.faction && (
                        <text className="node-faction-label" textAnchor="middle" dy="49" fill={color}>
                          {node.faction}
                        </text>
                      )}
                    </g>
                  )
                })}
              </g>
            </g>
          </svg>
        )}
      </div>

      {/* 侧边人物档案抽屉 */}
      <EntityDrawer
        projectId={projectId}
        node={selectedNode}
        allNodes={graphData?.nodes ?? []}
        edges={graphData?.edges ?? []}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false)
          setSelectedNode(null)
        }}
        onNodeUpdated={() => void loadGraph()}
        onEdgeUpdated={() => void loadGraph()}
      />

      {/* 手动添加实体弹窗 */}
      {showAddNodeModal && (
        <div className="modal-backdrop" onClick={() => setShowAddNodeModal(false)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '480px' }}>
            <h2 style={{ fontFamily: 'var(--font-serif)', fontSize: '1.25rem', marginTop: 0 }}>
              ➕ 新增小说实体
            </h2>
            <form className="stack" onSubmit={handleCreateManualNode}>
              <div className="two-col">
                <label>
                  实体名称
                  <input
                    value={newNodeName}
                    onChange={(e) => setNewNodeName(e.target.value)}
                    placeholder="如：韩立、落云宗、噬金虫"
                    required
                    autoFocus
                  />
                </label>
                <label>
                  实体类型
                  <select
                    value={newNodeKind}
                    onChange={(e) => setNewNodeKind(e.target.value as GraphNodeKind)}
                  >
                    <option value="character">🧑 人物角色</option>
                    <option value="faction">🏯 门派势力</option>
                    <option value="artifact">🔮 法宝神器</option>
                    <option value="location">🗺️ 地理秘境</option>
                  </select>
                </label>
              </div>

              <label>
                所属势力 / 门派阵营
                <input
                  value={newNodeFaction}
                  onChange={(e) => setNewNodeFaction(e.target.value)}
                  placeholder="如：黄枫谷、魔道六宗、极西之地"
                />
              </label>

              <label>
                核心定位与特征
                <textarea
                  value={newNodeSummary}
                  onChange={(e) => setNewNodeSummary(e.target.value)}
                  rows={3}
                  placeholder="人物性格、外貌特征或法宝用途简述"
                />
              </label>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.6rem', marginTop: '0.8rem' }}>
                <button
                  className="btn secondary sm"
                  type="button"
                  onClick={() => setShowAddNodeModal(false)}
                >
                  取消
                </button>
                <button
                  className="btn sm"
                  type="submit"
                  disabled={creatingNode || !newNodeName.trim()}
                >
                  {creatingNode ? '创建中…' : '确认创建'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
