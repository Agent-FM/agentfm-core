import { _electron as electron } from '@playwright/test'
import fs from 'node:fs'

const DESKTOP = '/Users/saif/Desktop/agentfm-prod/agentfm-core/agentfm-desktop'
const CMD = '/tmp/agentfm-drv.cmd'
const SHOTS = '/tmp/agentfm-shots'
fs.mkdirSync(SHOTS, { recursive: true })
fs.writeFileSync(CMD, '')

const app = await electron.launch({
  args: ['.'],
  cwd: DESKTOP,
  env: {
    ...process.env,
    AGENTFM_BIN: '/Users/saif/Desktop/agentfm-prod/agentfm-core/agentfm-go/agentfm',
  },
})
const page = await app.firstWindow()
await page.waitForLoadState('domcontentloaded')
console.log('LAUNCHED url=' + page.url())

async function shot(name) {
  const f = `${SHOTS}/${name}.png`
  await page.screenshot({ path: f, fullPage: true })
  return 'screenshot:' + f
}

let offset = 0
let busy = false
const pending = []

async function exec(id, js) {
  try {
    const fn = new Function('app', 'page', 'shot', `return (async () => { ${js} })()`)
    const r = await fn(app, page, shot)
    console.log(`DONE ${id}: ${typeof r === 'string' ? r : JSON.stringify(r)}`)
  } catch (e) {
    console.log(`FAIL ${id}: ${String(e.message || e).split('\n').slice(0, 3).join(' | ')}`)
  }
}

setInterval(() => {
  let st
  try { st = fs.statSync(CMD) } catch { return }
  if (st.size <= offset) return
  const fd = fs.openSync(CMD, 'r')
  const buf = Buffer.alloc(st.size - offset)
  fs.readSync(fd, buf, 0, buf.length, offset)
  fs.closeSync(fd)
  offset = st.size
  for (const line of buf.toString('utf8').split('\n')) {
    const m = line.match(/^(\w+)::([\s\S]*)$/)
    if (m) pending.push(m)
  }
  drain()
}, 250)

async function drain() {
  if (busy) return
  busy = true
  while (pending.length) {
    const [, id, js] = pending.shift()
    await exec(id, js)
  }
  busy = false
}

app.process().on('exit', () => {
  console.log('APP_EXITED')
  process.exit(0)
})
