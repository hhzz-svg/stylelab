// Colours for the communities Louvain finds, by community index. Distinct
// from each other on the dark canvas; a node in no community is grey.
export const COMMUNITY_COLORS = [
  '#f7cb68', // 金
  '#2dd4bf', // 碧
  '#f87171', // 赤
  '#a78bfa', // 紫
  '#38bdf8', // 蓝
  '#fb923c', // 橙
  '#4ade80', // 翠
  '#f472b6', // 粉
]

export const LONER_COLOR = '#64748b'

export function communityColor(index: number | undefined): string {
  return index === undefined ? LONER_COLOR : COMMUNITY_COLORS[index % COMMUNITY_COLORS.length]
}
