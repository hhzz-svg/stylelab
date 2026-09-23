// A stand-in for the model provider. Users' BYOK keys point their base_url
// here, so the real job handlers run end to end. Replies are picked by the
// system prompt and held back a few seconds, long enough to watch a job run.
import http from 'node:http'

const port = Number(process.env.E2E_STUB_PORT ?? 8198)
const delay = Number(process.env.E2E_STUB_DELAY_MS ?? 3000)

const graph = JSON.stringify({
  nodes: [
    { name: '林远', kind: 'character', faction: '青云宗', summary: '主角' },
    { name: '苏晚', kind: 'character', faction: '青云宗', summary: '师姐' },
  ],
  edges: [{ source: '林远', target: '苏晚', relation: '同门', description: '' }],
})

const outline = JSON.stringify({
  synopsis: '测试梗概',
  volumes: [
    {
      volume_index: 1,
      volume_title: '第一卷',
      volume_brief: '开局',
      chapters: [{ title: '第1章', brief: '主角出场', hook: '' }],
    },
  ],
})

function replyFor(system) {
  if (system.includes('实体图谱')) return graph
  if (system.includes('分卷大纲')) return outline
  return '风从北边来。'
}

http
  .createServer((req, res) => {
    if (req.url === '/health') {
      res.end('ok')
      return
    }
    let body = ''
    req.on('data', (c) => (body += c))
    req.on('end', () => {
      let system = ''
      try {
        system = JSON.parse(body).messages?.find((m) => m.role === 'system')?.content ?? ''
      } catch {
        // not a chat request; answer with prose
      }
      setTimeout(() => {
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify({ choices: [{ message: { content: replyFor(system) } }] }))
      }, delay)
    })
  })
  .listen(port, '127.0.0.1', () => console.log(`stub llm on ${port}`))
