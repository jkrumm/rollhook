import path from 'node:path'
import { Elysia } from 'elysia'
import { api } from './src/api'

const isProd = process.env.NODE_ENV === 'production'
const WEB_ROOT = path.resolve(import.meta.dir, '../web')

const app = new Elysia().use(api)

if (isProd) {
  const { staticPlugin } = await import('@elysiajs/static')
  app.use(
    staticPlugin({
      assets: path.join(WEB_ROOT, 'dist/client'),
      prefix: '/',
      alwaysStatic: true,
    }),
  )
  const { default: handler } = await import(path.join(WEB_ROOT, 'dist/server/server.js'))
  app.all('*', ({ request }) => handler.fetch(request))
}
else {
  const { createServer } = await import('vite')
  const { connect } = await import('elysia-connect-middleware')
  const viteDevServer = await createServer({ root: WEB_ROOT, server: { middlewareMode: true } })
  app.use(connect(viteDevServer.middlewares))
  app.all('*', async ({ request }) => {
    try {
      const { default: serverEntry } = await viteDevServer.ssrLoadModule('./src/server.ts')
      return serverEntry.fetch(request)
    }
    catch (e) {
      if (e instanceof Error)
        viteDevServer.ssrFixStacktrace(e)
      console.error(e)
      return new Response('Internal Server Error', { status: 500 })
    }
  })
}

app.listen(Number(process.env.PORT ?? 7700), () =>
  console.log(`StackCommander running on http://localhost:${process.env.PORT ?? 7700}`),
)

export type App = typeof app
