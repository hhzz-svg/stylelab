import { expect, test } from '@playwright/test'
import { addChapter, addEdge, addNode, setup } from './helpers'

// The graph analyses (internal/insight) as the author sees them.

test('importance ranking puts the best-connected character first and sizes nodes by it', async ({ page }) => {
  const projectId = await setup(page)
  const centre = await addNode(page, projectId, { name: '林远', kind: 'character' })
  for (const name of ['苏晚', '赵四', '钱五', '孙六']) {
    await addEdge(page, projectId, centre, await addNode(page, projectId, { name, kind: 'character' }))
  }

  await page.goto(`/p/${projectId}/graph`)
  const rows = page.locator('.insight-panel .rank-row')
  await expect(rows).toHaveCount(5)
  await expect(rows.first()).toContainText('林远')
  await expect(rows.first().locator('.role-chip.core')).toHaveText('核心')
  await expect(rows.first().locator('.rank-score')).toHaveText('100')

  // The top node is drawn 1.3x: its core circle is 18 * 1.3.
  const core = page.locator('.graph-node-group', { hasText: '林远' }).locator('.node-core-circle')
  const radius = async () => Number(await core.getAttribute('r'))
  await expect.poll(radius).toBeCloseTo(23.4, 5)
  await page.getByTitle('节点大小按重要度显示').click()
  await expect.poll(radius).toBeCloseTo(18, 5)

  // Clicking a row opens that character's profile.
  await rows.first().click()
  await expect(page.locator('.entity-drawer-title')).toHaveText('林远')
})

test('community detection groups the camps and points out the spy', async ({ page }) => {
  const projectId = await setup(page)
  const add = (name: string, faction: string) => addNode(page, projectId, { name, kind: 'character', faction })
  const q = [await add('林远', '青云宗'), await add('苏晚', '青云宗'), await add('赵长老', '青云宗'), await add('卧底', '魔门')]
  const m = [await add('魔尊', '魔门'), await add('血影', '魔门'), await add('鬼婆', '魔门')]
  for (const group of [q, m]) {
    for (let i = 0; i < group.length; i++) {
      for (let j = i + 1; j < group.length; j++) await addEdge(page, projectId, group[i], group[j])
    }
  }
  await addEdge(page, projectId, q[0], m[0])

  await page.goto(`/p/${projectId}/graph`)
  const cards = page.locator('.community-card')
  await expect(cards).toHaveCount(2)
  await expect(cards.nth(0).locator('.community-head')).toContainText('青云宗')
  await expect(cards.nth(1).locator('.community-head')).toContainText('魔门')
  await expect(cards.nth(0).locator('.community-outlier')).toContainText('卧底')
  await expect(cards.nth(0).locator('.community-outlier')).toContainText('标注为「魔门」')

  // Coloured by labelled faction the spy looks like 魔门; coloured by the
  // detected groups he takes 林远's colour.
  const ring = (name: string) =>
    page.locator('.graph-node-group', { hasText: name }).locator('.node-glow-ring')
  const stroke = async (name: string) => ring(name).getAttribute('stroke')
  expect(await stroke('卧底')).toBe(await stroke('魔尊'))
  await page.getByTitle('按算法发现的阵营着色').click()
  await expect.poll(() => stroke('卧底')).toBe(await stroke('林远'))
  expect(await stroke('卧底')).not.toBe(await stroke('魔尊'))
})

test('lineage tree puts the master above the disciple, and a swap flips them', async ({ page }) => {
  const projectId = await setup(page)
  await addNode(page, projectId, { name: '青云宗', kind: 'faction' })
  const master = await addNode(page, projectId, { name: '赵长老', kind: 'character', faction: '青云宗' })
  const disciple = await addNode(page, projectId, { name: '林远', kind: 'character', faction: '青云宗' })
  await addEdge(page, projectId, master, disciple, '师徒')
  await addNode(page, projectId, { name: '血影', kind: 'character', faction: '魔门' })

  await page.goto(`/p/${projectId}/graph`)
  await page.getByRole('tab', { name: /谱系树/ }).click()

  const top = async (id: string) => (await page.locator(`.lineage-node[data-id="${id}"]`).boundingBox())!.y
  await expect(page.locator(`.lineage-node[data-id="${disciple}"]`)).toBeVisible()
  expect(await top(master)).toBeLessThan(await top(disciple))
  await expect(page.locator(`.lineage-node[data-id="${disciple}"] .lineage-relation`)).toHaveText('师徒')
  // 魔门 has no node of its own: it stands as a dashed virtual root.
  await expect(page.locator('.lineage-node.virtual', { hasText: '魔门' })).toBeVisible()

  // Open the disciple's profile from the tree and swap the relation.
  await page.locator(`.lineage-node[data-id="${disciple}"]`).click()
  await expect(page.locator('.entity-drawer-title')).toHaveText('林远')
  await page.getByTitle('调换方向（谱系树里上下级互换）').click()
  await expect.poll(async () => (await top(disciple)) < (await top(master))).toBe(true)
})

test('appearance timeline counts aliases, flags the absent and suggests missing relations', async ({ page }) => {
  const projectId = await setup(page)
  await addNode(page, projectId, { name: '林远', kind: 'character', details: { aliases: ['林师兄'] } })
  const su = await addNode(page, projectId, { name: '苏晚', kind: 'character' })
  await addNode(page, projectId, { name: '魔尊', kind: 'character' })
  await addNode(page, projectId, { name: '赵四', kind: 'character' })
  await addChapter(page, projectId, '初遇', '林远与苏晚同行。\n魔尊闭关，魔尊不语，魔尊入定。')
  await addChapter(page, projectId, '护送', '林师兄护着苏晚。\n魔尊出关，魔尊冷笑。')
  await addChapter(page, projectId, '疗伤', '晚儿替林远包扎。')
  // 林远 travels with 赵四 twice, which makes him the hub of the prose.
  for (let i = 4; i <= 13; i++) await addChapter(page, projectId, `赶路${i}`, i <= 5 ? '林远与赵四赶路。' : '林远赶路。')

  await page.goto(`/p/${projectId}/graph`)
  await page.getByRole('tab', { name: /出场时间线/ }).click()

  const row = (name: string) => page.locator('.timeline-grid tbody tr', { hasText: name })
  await expect(page.locator('.timeline-grid tbody tr')).toHaveCount(4)
  // The alias 林师兄 counts for 林远 in chapter 2.
  await expect(row('林远').locator('td').nth(1)).toHaveAttribute('data-count', '1')
  await expect(row('苏晚').locator('.timeline-total')).toHaveText('2')
  await expect(page.locator('.timeline-side')).toContainText('魔尊：已 11 章没有出场（最后在第 2 章）')
  await expect(page.locator('.suggest-row')).toHaveCount(0)

  // Give 苏晚 the nickname the prose uses: her count and her tie to 林远 grow.
  await row('苏晚').getByRole('button', { name: '苏晚' }).click()
  await page.getByPlaceholder(/正文里的其他叫法/).fill('晚儿')
  await page.getByRole('button', { name: /保存档案/ }).click()
  await expect(row('苏晚').locator('.timeline-total')).toHaveText('3')
  await page.locator('.entity-drawer-wrap').click({ position: { x: 20, y: 20 } })

  const suggestion = page.locator('.suggest-row')
  await expect(suggestion).toHaveCount(1)
  await expect(suggestion).toContainText('林远 × 苏晚')
  await expect(suggestion).toContainText('同场 3 次')
  await suggestion.getByRole('button', { name: '建立关系' }).click()
  await expect(suggestion).toHaveCount(0)
  const edges = (await (await page.request.get(`/api/projects/${projectId}/graph`)).json()).edges
  expect(edges.some((e: { target_id: string; relation: string }) => e.target_id === su && e.relation === '同场')).toBe(true)

  // In the network view, weighting by the prose makes 林远 the core.
  await page.getByRole('tab', { name: /关系网/ }).click()
  await page.getByRole('radio', { name: '正文共现' }).click()
  await expect(page.locator('.insight-panel .rank-row').first()).toContainText('林远')
})
