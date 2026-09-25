import { expect, test } from '@playwright/test'
import { THREE_SCENES, addChapter, setup } from './helpers'

test('the chapter list becomes a volume -> chapter -> scene tree', async ({ page }) => {
  const projectId = await setup(page)
  const first = await addChapter(page, projectId, '出山', THREE_SCENES)
  for (const t of ['拜师', '下山', '入城', '夜战']) await addChapter(page, projectId, t, '')
  expect((await page.request.post(`/api/chapters/${first}/scenes/split`, { data: {} })).ok()).toBe(true)

  await page.goto(`/p/${projectId}/write`)
  await expect(page.locator('.chapter-row')).toHaveCount(5)
  await expect(page.locator('.volume-head')).toHaveCount(0) // no volumes yet: a plain list

  // Scenes show under their chapter.
  await expect(page.locator('.chapter-row').first().locator('.row-scenes li')).toHaveCount(3)

  // Start a volume at chapter 1, then another at chapter 4.
  const newVolumeAt = async (row: number, title: string) => {
    await page.locator('.chapter-row').nth(row).getByTitle('从这一章开始新的一卷').click()
    await page.getByLabel('卷名').fill(title)
    await page.getByRole('button', { name: '保存' }).click()
  }
  await newVolumeAt(0, '第一卷 · 出山')
  await expect(page.locator('.volume-head')).toHaveCount(1)
  await expect(page.locator('.volume-head')).toContainText('第 1–5 章 · 5 章 · 已写 1')
  await newVolumeAt(3, '第二卷 · 入城')
  const heads = page.locator('.volume-head')
  await expect(heads).toHaveCount(2)
  await expect(heads.nth(0)).toContainText('第 1–3 章')
  await expect(heads.nth(1)).toContainText('第二卷 · 入城')
  await expect(heads.nth(1)).toContainText('第 4–5 章')
  // A volume already starts at chapter 1: no second one offered there.
  await expect(page.locator('.chapter-row').first().getByTitle('从这一章开始新的一卷')).toHaveCount(0)

  // Collapsing a volume hides its chapters.
  await heads.nth(0).getByRole('button', { name: /第一卷/ }).click()
  await expect(page.locator('.chapter-row')).toHaveCount(2)
  await heads.nth(0).getByRole('button', { name: /第一卷/ }).click()
  await expect(page.locator('.chapter-row')).toHaveCount(5)

  // Deleting chapter 2 moves everything after it up; the second volume
  // still starts at 入城.
  const chapters = (await (await page.request.get(`/api/projects/${projectId}/chapters`)).json()).chapters
  expect((await page.request.delete(`/api/chapters/${chapters[1].id}`)).ok()).toBe(true)
  await page.reload()
  await expect(heads.nth(1)).toContainText('第 3–4 章')
  await expect(page.locator('.volume-group').nth(1).locator('.chapter-row').first()).toContainText('入城')
})

test('importing a generated outline keeps its volumes', async ({ page }) => {
  const projectId = await setup(page)
  await page.goto(`/p/${projectId}/write`)
  await page.getByRole('button', { name: /智能大纲规划/ }).click()
  await page.locator('.modal-panel textarea').first().fill('一个少年拾得残镜，踏上修行之路，最终揭开古灯背后的真相。')
  await page.getByRole('button', { name: /开始智能推演大纲/ }).click()
  await page.getByRole('button', { name: /一键批量导入目录/ }).click({ timeout: 20_000 })

  // The stub's outline has one volume, 第一卷, holding one chapter.
  await expect(page.locator('.volume-head')).toHaveCount(1)
  await expect(page.locator('.volume-head')).toContainText('第一卷')
  await expect(page.locator('.volume-head')).toContainText('第 1 章 · 1 章')
})
