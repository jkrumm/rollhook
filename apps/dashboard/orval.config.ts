import { defineConfig } from 'orval'

export default defineConfig({
  rollhook: {
    input: {
      // Static spec committed alongside generated output.
      // Regenerate with: go run ./cmd/gendocs > apps/dashboard/openapi.json
      target: './openapi.json',
    },
    output: {
      mode: 'tags-split',
      target: 'src/api/generated',
      schemas: 'src/api/generated/models',
      client: 'fetch',
      override: {
        mutator: {
          path: 'src/api/client.ts',
          name: 'customInstance',
        },
      },
    },
  },
})
