import { treaty } from '@elysiajs/eden'
import type { App } from '@rollhook/server/app'

// Browser-side HTTP client — used for client-side mutations
export const api = treaty<App>('localhost:7700')
