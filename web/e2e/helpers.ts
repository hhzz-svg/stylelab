import { expect, type Page } from '@playwright/test'
import { STUB_URL } from './env'

/** Registers a fresh user with a project and a BYOK key pointed at the stub. */
export async function setup(page: Page, chapters = 0): Promise<string> {
  const api = page.request
  const email = `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`
  expect((await api.post('/api/auth/register', { data: { email, password: 'password1' } })).ok()).toBe(true)
  const project = await api.post('/api/projects', { data: { name: '冒烟测试' } })
  expect(project.ok()).toBe(true)
  const projectId = (await project.json()).id as string
  for (let i = 1; i <= chapters; i++) {
    const ch = await api.post(`/api/projects/${projectId}/chapters`, { data: { title: `章${i}`, brief: `第${i}章梗概` } })
    expect(ch.ok()).toBe(true)
    const patched = await api.patch(`/api/chapters/${(await ch.json()).id}`, { data: { body: '林远与苏晚同门学艺。' } })
    expect(patched.ok()).toBe(true)
  }
  const key = await api.put('/api/me/llm-keys', {
    data: { provider: 'chat', base_url: STUB_URL, api_key: 'sk-test-abcd' },
  })
  expect(key.ok()).toBe(true)
  return projectId
}

/** Adds a graph node through the API and returns its id. */
export async function addNode(page: Page, projectId: string, node: Record<string, unknown>): Promise<string> {
  const res = await page.request.post(`/api/projects/${projectId}/graph/nodes`, { data: node })
  expect(res.ok()).toBe(true)
  return (await res.json()).id as string
}

/** Adds a graph relation through the API. */
export async function addEdge(page: Page, projectId: string, source: string, target: string, relation = '同门') {
  const res = await page.request.post(`/api/projects/${projectId}/graph/edges`, {
    data: { source_id: source, target_id: target, relation },
  })
  expect(res.ok()).toBe(true)
}

/** Adds a chapter with the given prose and returns its id. */
export async function addChapter(page: Page, projectId: string, title: string, body: string): Promise<string> {
  const ch = await page.request.post(`/api/projects/${projectId}/chapters`, { data: { title, brief: '梗概' } })
  expect(ch.ok()).toBe(true)
  const id = (await ch.json()).id as string
  const res = await page.request.patch(`/api/chapters/${id}`, { data: { body } })
  expect(res.ok()).toBe(true)
  return id
}

// Three scenes' worth of prose -- the valley, 次日 the library, 与此同时 the
// palace -- each long enough (over 300 runes) to stand as a scene.
const VALLEY = [
  '林远在山谷中练剑，剑光如水，剑气纵横，惊起一片飞鸟。',
  '他收剑而立，山风拂过山谷，谷中松涛阵阵，剑穗轻摇。',
  '林远再次拔剑，剑锋所指，山石崩裂，碎石纷飞，剑气久久不散。',
  '剑意越来越凝练，林远的呼吸也越来越沉稳，剑与人仿佛合为一体。',
  '山谷深处的瀑布轰鸣，水雾打湿了他的衣衫，他却浑然不觉，只管练剑。',
  '一套剑法使完，林远收剑入鞘，望着山谷里被剑气削平的巨石出神。',
]
const LIBRARY = [
  '苏晚在藏经阁里翻阅古籍，书页泛黄，墨香扑鼻，烛火映着她的侧脸。',
  '她一卷卷地查找，指尖拂过书脊，古籍上的字迹早已模糊不清。',
  '藏经阁的书架高耸入顶，苏晚踩着木梯，从最上层抽出一本残破的典籍。',
  '典籍里记载着失传的阵法，苏晚看得入神，连烛泪流尽都没有察觉。',
  '她把要紧的段落抄在纸上，又将古籍小心放回原处，拂去书上的灰尘。',
  '夜读至此，藏经阁外传来更鼓声，苏晚合上书卷，揉了揉酸涩的眼睛。',
]
const PALACE = [
  '魔宫深处，魔尊端坐王座，血色灯火摇曳，照得殿中一片猩红。',
  '殿下跪着数名魔将，无人敢抬头，魔宫里只听得见灯芯爆裂的声音。',
  '魔尊缓缓开口，声音冰冷，命魔将们三日之内踏平正道的山门。',
  '魔将们领命退下，魔宫的大门轰然关闭，血色的雾气翻涌不息。',
  '魔尊独自坐在王座上，指尖敲着扶手，眼中闪过一丝阴鸷的杀意。',
  '王座之后的血池翻滚冒泡，魔尊的影子在猩红灯火中拉得很长。',
]
const section = (cue: string, paras: string[]) => {
  const out = [...paras, ...paras]
  out[0] = cue + out[0]
  return out.join('\n')
}
export const THREE_SCENES = [section('', VALLEY), section('次日，', LIBRARY), section('与此同时，', PALACE)].join('\n')
