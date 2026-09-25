import { expect, test, type Page } from '@playwright/test'
import { addChapter, addNode, setup } from './helpers'

// Each test pins a bug that shipped once and was only caught by hand in a
// browser. Unit tests cannot see these: they are about layout and about
// what the user is told, not about what the API returns.

/** Waits out entry and slide-in animations; spinners loop forever, so skip those. */
async function settleAnimations(page: Page) {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  )
}

/** What a click at (x, y) would land on. */
async function hitClass(page: Page, x: number, y: number): Promise<string> {
  return page.evaluate(([px, py]) => document.elementFromPoint(px, py)?.className.toString() ?? '', [x, y])
}

/**
 * Records every toast shown in each document the page loads. Toasts dismiss
 * themselves after a few seconds, so counting the ones on screen at the end
 * can miss a duplicate that has already gone.
 */
async function recordToasts(page: Page) {
  await page.addInitScript(() => {
    const seen: string[] = []
    ;(window as unknown as { __toasts: string[] }).__toasts = seen
    new MutationObserver((records) => {
      for (const r of records) {
        r.addedNodes.forEach((n) => {
          if (n instanceof HTMLElement && n.classList.contains('toast')) {
            seen.push(n.querySelector('.toast-msg')?.textContent ?? '')
          }
        })
      }
    }).observe(document, { childList: true, subtree: true })
  })
}

/** Toasts shown so far, after waiting out the dock's grace period. */
async function shownToasts(page: Page): Promise<string[]> {
  await page.waitForTimeout(2500)
  return page.evaluate(() => (window as unknown as { __toasts: string[] }).__toasts)
}

test('graph extraction runs as a job, survives a reload, and is announced once', async ({ page }) => {
  const projectId = await setup(page, 3)
  await recordToasts(page)
  await page.goto(`/p/${projectId}/graph`)
  await page.getByRole('button', { name: /AI 从正文提炼图谱/ }).click()

  // Every progress bar used to be a zero-height div.
  const fill = page.locator('.job-bar .job-fill')
  await expect(fill).toBeVisible()
  expect((await fill.boundingBox())?.height ?? 0).toBeGreaterThan(0)
  await expect(page.locator('.job-bar p')).toHaveText(/\d+%/)

  // A reload keeps the job: the dock holds it and the page re-attaches.
  await page.reload()
  await expect(page.locator('.job-dock')).toContainText('AI 提炼图谱')
  await expect(page.locator('.job-bar')).toBeVisible()

  await expect(page.locator('svg text').filter({ hasText: /^林远$/ })).toBeVisible({ timeout: 20_000 })
  // One toast, the page's own with the counts: not two copies of it
  // (ToastHost was mounted twice), and not the dock's generic one as well.
  const toasts = await shownToasts(page)
  expect(toasts).toHaveLength(1)
  expect(toasts[0]).toContain('AI 提炼完成')
})

test('the dock announces a job whose page was left', async ({ page }) => {
  const projectId = await setup(page, 1)
  await recordToasts(page)
  await page.goto(`/p/${projectId}/graph`)
  await page.getByRole('button', { name: /AI 从正文提炼图谱/ }).click()
  await expect(page.locator('.job-bar')).toBeVisible()

  await page.goto(`/p/${projectId}/write`)
  // Nobody on this page claimed the job, so the dock must say it finished.
  await expect(page.locator('.toast').first()).toBeVisible({ timeout: 20_000 })
  expect(await shownToasts(page)).toEqual(['AI 提炼图谱完成'])
})

test('studio modals cover the whole viewport', async ({ page }) => {
  const projectId = await setup(page)
  await page.goto(`/p/${projectId}/write`)
  await page.getByRole('button', { name: /智能大纲规划/ }).click()
  await settleAnimations(page)

  // The page's entry animation once left a transform behind that made .page
  // the containing block, so fixed overlays were clipped to the content area.
  const box = await page.locator('.modal-backdrop').boundingBox()
  expect(box).toEqual({ x: 0, y: 0, width: 1440, height: 1000 })
  // And it is on top: the sidebar is covered, not left clickable above it.
  expect(await hitClass(page, 100, 500)).toBe('modal-backdrop')
})

test('the card deck opens as a full-height drawer', async ({ page }) => {
  const projectId = await setup(page)
  await page.goto(`/p/${projectId}`)
  await page.getByRole('button', { name: /牌库/ }).click()
  await settleAnimations(page)

  // It used to render as a block at the foot of the page, with a zero-size
  // scrim so clicking outside did nothing.
  const drawer = await page.locator('.deck-drawer').boundingBox()
  expect(drawer).not.toBeNull()
  expect(drawer!.y).toBe(0)
  expect(drawer!.height).toBe(1000)
  expect(drawer!.x + drawer!.width).toBe(1440)
  expect(drawer!.width).toBeLessThanOrEqual(460)
  expect(await page.locator('.deck-scrim').boundingBox()).toEqual({ x: 0, y: 0, width: 1440, height: 1000 })
  expect(await hitClass(page, 100, 500)).toBe('deck-scrim')

  await page.locator('.deck-scrim').click({ position: { x: 100, y: 500 } })
  await expect(page.locator('.deck-drawer')).toHaveCount(0)
})

test('on a phone the opened sidebar sits above the page', async ({ page }) => {
  // Guards the .table stacking fix from the other side: page overlays now
  // share a stacking context with the sidebar, and the page itself must not
  // cover the slide-out menu.
  await page.setViewportSize({ width: 390, height: 844 })
  const projectId = await setup(page, 1)
  await page.goto(`/p/${projectId}/write`)
  await page.locator('.mobile-bar .menu-btn').first().click()
  await settleAnimations(page)
  const inSidebar = await page.evaluate(() => !!document.elementFromPoint(130, 400)?.closest('.sidebar'))
  expect(inSidebar).toBe(true)
})

test('a wide appearance timeline scrolls inside its box, not the page', async ({ page }) => {
  // The app grid's 1fr column had a min-content floor: 60 chapter columns
  // widened the whole page and pushed the header off screen.
  const projectId = await setup(page)
  await addNode(page, projectId, { name: '林远', kind: 'character' })
  for (let i = 1; i <= 60; i++) await addChapter(page, projectId, `第${i}章`, '林远赶路。')
  await page.goto(`/p/${projectId}/graph`)
  await page.getByRole('tab', { name: /出场时间线/ }).click()
  await expect(page.locator('.timeline-grid tbody tr')).toHaveCount(1)

  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
  expect(overflow).toBeLessThanOrEqual(0)
  const box = await page.locator('.timeline-scroll').evaluate((el) => ({ client: el.clientWidth, scroll: el.scrollWidth }))
  expect(box.scroll).toBeGreaterThan(box.client)
})
