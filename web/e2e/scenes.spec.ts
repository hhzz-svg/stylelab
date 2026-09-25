import { expect, test } from '@playwright/test'
import { THREE_SCENES, addChapter, addNode, setup } from './helpers'

test('a chapter splits into scenes that locate their text and take a new title', async ({ page }) => {
  const projectId = await setup(page)
  for (const [name, kind] of [['林远', 'character'], ['苏晚', 'character'], ['魔尊', 'character'], ['藏经阁', 'location']]) {
    await addNode(page, projectId, { name, kind })
  }
  const chapterId = await addChapter(page, projectId, '三处', THREE_SCENES)

  await page.goto(`/p/${projectId}/chapter/${chapterId}`)
  const panel = page.locator('.scene-panel')
  await panel.getByRole('button', { name: /切分场景/ }).click()
  const cards = panel.locator('.scene-card')
  await expect(cards).toHaveCount(3)
  await expect(cards.nth(1).locator('.role-chip')).toHaveText('次日')
  await expect(cards.nth(2).locator('.role-chip')).toHaveText('与此同时')
  await expect(cards.nth(1).locator('.scene-meta')).toContainText('📍 藏经阁')
  await expect(cards.nth(1).locator('.scene-meta')).toContainText('🧑 苏晚')

  // Clicking a scene selects exactly its text in the editor.
  await cards.nth(1).locator('.scene-title').click()
  const selected = await page.locator('textarea.chapter-body').evaluate((el) => {
    const ta = el as HTMLTextAreaElement
    return ta.value.slice(ta.selectionStart, ta.selectionEnd)
  })
  expect(selected.startsWith('次日，苏晚在藏经阁')).toBe(true)
  expect(selected.endsWith('揉了揉酸涩的眼睛。')).toBe(true)

  // Rename it; the new title survives a reload.
  await page.getByRole('button', { name: '改场景 2 的标题' }).click()
  await page.getByLabel('场景标题').fill('藏经阁夜读')
  await page.getByLabel('场景标题').press('Enter')
  await expect(cards.nth(1).locator('.scene-title')).toHaveText('藏经阁夜读')
  await page.reload()
  await expect(page.locator('.scene-card').nth(1).locator('.scene-title')).toHaveText('藏经阁夜读')

  // The timeline now counts by scene for this book.
  await page.goto(`/p/${projectId}/graph`)
  await page.getByRole('tab', { name: /出场时间线/ }).click()
  await expect(page.locator('.timeline-head')).toContainText('3 个场景')
})

test('the timeline can split the whole book into scenes', async ({ page }) => {
  const projectId = await setup(page)
  await addNode(page, projectId, { name: '林远', kind: 'character' })
  await addChapter(page, projectId, '一', THREE_SCENES)
  await addChapter(page, projectId, '二', '林远出门了。')

  await page.goto(`/p/${projectId}/graph`)
  await page.getByRole('tab', { name: /出场时间线/ }).click()
  await expect(page.locator('.timeline-head')).toContainText('段落')
  await page.getByRole('button', { name: /全书切分场景/ }).click()
  await expect(page.locator('.timeline-head')).toContainText('2 章 · 4 个场景')
  await expect(page.getByRole('button', { name: /全书切分场景/ })).toHaveCount(0)
})
