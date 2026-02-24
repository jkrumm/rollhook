import process from 'node:process'

const VERSION = process.env.BUILD_VERSION ?? 'unknown'

Bun.serve({
  port: Number(process.env.PORT ?? 3000),
  fetch(req) {
    const { pathname } = new URL(req.url)
    if (pathname === '/health')
      return new Response('ok')
    if (pathname === '/version')
      return Response.json({ version: VERSION, pid: process.pid })
    return new Response('not found', { status: 404 })
  },
})
