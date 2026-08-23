/**
 * 作品名缓存：任何页面加载到作品信息时回填，面包屑据此显示名称。
 * 会话内有效，未命中时退回通用文案。
 */

const cache = new Map<string, string>()

export function rememberProject(id: string, name: string) {
  if (id && name) cache.set(id, name)
}

export function projectName(id: string): string {
  return cache.get(id) ?? '作品'
}
