import { openapi } from '@elysiajs/openapi'
import { Elysia } from 'elysia'
import { api } from './api.js'

export const app = new Elysia()
  .use(openapi({
    path: '/openapi',
    documentation: {
      info: { title: 'RollHook API', version: '0.0.0' },
      tags: [{ name: 'Counter', description: 'Counter endpoints' }],
    },
  }))
  .use(api)

export type App = typeof app
