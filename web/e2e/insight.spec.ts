import { expect, test } from '@playwright/test'
import { addEdge, addNode, setup } from './helpers'

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
