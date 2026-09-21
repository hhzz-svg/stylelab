#!/usr/bin/env node
// Guards the two failure modes that shipped silently in commit 9cf4fb7:
//
//   1. A component using var(--border) when the design system defines --line.
//      An undefined custom property is not an error in CSS -- the declaration
//      is simply dropped -- so borders vanished and nothing anywhere complained.
//
//   2. A component using .modal-backdrop when styles.css never defined it.
//      Without position:fixed the two studio modals rendered as ordinary blocks
//      at the foot of the page, and every spinner sat frozen.
//
// Scope: this only sees STATIC class names. A className built by string
// concatenation or from a variable is invisible here, so a clean run means
// "no statically detectable gap", not "every class is styled".
//
// Usage: node scripts/check-css.mjs [--write-baseline]

import { readFileSync, writeFileSync, readdirSync, statSync } from 'node:fs'
import { join, dirname, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const webRoot = join(dirname(fileURLToPath(import.meta.url)), '..')
const srcRoot = join(webRoot, 'src')
const stylesPath = join(srcRoot, 'styles.css')
const baselinePath = join(webRoot, 'scripts', 'css-baseline.json')

function sourceFiles(dir) {
  const out = []
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) {
      out.push(...sourceFiles(full))
    } else if (/\.(ts|tsx)$/.test(entry)) {
      out.push(full)
    }
  }
  return out
}

const css = readFileSync(stylesPath, 'utf8')
const files = sourceFiles(srcRoot)

// --- what the stylesheet provides ------------------------------------------

const definedClasses = new Set()
for (const m of css.matchAll(/\.(-?[A-Za-z_][\w-]*)/g)) definedClasses.add(m[1])

const definedVars = new Set()
for (const m of css.matchAll(/(--[\w-]+)\s*:/g)) definedVars.add(m[1])

// --- what the code asks for -------------------------------------------------

/** @type {Map<string, Set<string>>} */
const usedClasses = new Map()
/** @type {Map<string, Set<string>>} */
const usedVars = new Map()

function record(map, key, file) {
  if (!map.has(key)) map.set(key, new Set())
  map.get(key).add(relative(webRoot, file))
}

for (const file of [...files, stylesPath]) {
  const text = readFileSync(file, 'utf8')

  for (const m of text.matchAll(/var\(\s*(--[\w-]+)/g)) {
    record(usedVars, m[1], file)
  }

  if (file === stylesPath) continue

  // className="a b c"
  for (const m of text.matchAll(/className\s*=\s*"([^"{}]*)"/g)) {
    for (const cls of m[1].split(/\s+/).filter(Boolean)) record(usedClasses, cls, file)
  }
  // className={`a ${cond} c`} -- only the literal segments are knowable
  for (const m of text.matchAll(/className\s*=\s*\{`([^`]*)`\}/g)) {
    const literal = m[1].replace(/\$\{[^}]*\}/g, ' ')
    for (const cls of literal.split(/\s+/).filter(Boolean)) record(usedClasses, cls, file)
  }
}

// A trailing fragment like `kind-` comes from `kind-${x}`; the full name is
// built at runtime and cannot be checked statically.
const isPartial = (cls) => cls.endsWith('-')

const missingVars = [...usedVars.keys()].filter((v) => !definedVars.has(v)).sort()
const missingClasses = [...usedClasses.keys()]
  .filter((c) => !definedClasses.has(c) && !isPartial(c))
  .sort()

// --- baseline ---------------------------------------------------------------

if (process.argv.includes('--write-baseline')) {
  const baseline = {
    comment:
      'Class names used in src/ with no rule in styles.css, as of the commit that added this check. ' +
      'These are pre-existing gaps, not approved ones -- the list must only ever shrink. ' +
      'Regenerate with: npm run lint:css -- --write-baseline',
    classes: missingClasses,
  }
  writeFileSync(baselinePath, JSON.stringify(baseline, null, 2) + '\n')
  console.log(`wrote baseline with ${missingClasses.length} known-missing classes`)
  process.exit(0)
}

let baseline = { classes: [] }
try {
  baseline = JSON.parse(readFileSync(baselinePath, 'utf8'))
} catch {
  // no baseline yet: every gap is new
}
const known = new Set(baseline.classes ?? [])

const newClasses = missingClasses.filter((c) => !known.has(c))
const fixedClasses = [...known].filter((c) => !missingClasses.includes(c)).sort()

// --- report -----------------------------------------------------------------

let failed = false

if (missingVars.length) {
  failed = true
  console.error(`\n✗ ${missingVars.length} undefined CSS custom propert${missingVars.length === 1 ? 'y' : 'ies'}:`)
  for (const v of missingVars) {
    console.error(`    ${v}  <- ${[...usedVars.get(v)].sort().join(', ')}`)
  }
  console.error('\n  These resolve to nothing at runtime and fail silently.')
  console.error(`  Use a property defined in ${relative(webRoot, stylesPath)} (e.g. --line, --ink, --ink-soft).`)
}

if (newClasses.length) {
  failed = true
  console.error(`\n✗ ${newClasses.length} class name(s) used but never defined in styles.css:`)
  for (const c of newClasses) {
    console.error(`    .${c}  <- ${[...usedClasses.get(c)].sort().join(', ')}`)
  }
  console.error('\n  Add a rule for each, or -- if it is intentionally unstyled --')
  console.error('  record it with: npm run lint:css -- --write-baseline')
}

if (fixedClasses.length) {
  failed = true
  console.error(`\n✗ ${fixedClasses.length} baseline entr${fixedClasses.length === 1 ? 'y is' : 'ies are'} now defined:`)
  for (const c of fixedClasses) console.error(`    .${c}`)
  console.error('\n  Good -- now shrink the baseline: npm run lint:css -- --write-baseline')
}

if (failed) {
  process.exit(1)
}

console.log(
  `css check passed - ${definedClasses.size} classes and ${definedVars.size} custom properties defined; ` +
    `${usedVars.size} properties used, all defined; ${known.size} known-missing class(es) in the baseline.`,
)
