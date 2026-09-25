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

/** Adds a chapter with the given prose. */
export async function addChapter(page: Page, projectId: string, title: string, body: string) {
  const ch = await page.request.post(`/api/projects/${projectId}/chapters`, { data: { title, brief: '梗概' } })
  expect(ch.ok()).toBe(true)
  const res = await page.request.patch(`/api/chapters/${(await ch.json()).id}`, { data: { body } })
  expect(res.ok()).toBe(true)
}
