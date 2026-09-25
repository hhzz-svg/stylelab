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
