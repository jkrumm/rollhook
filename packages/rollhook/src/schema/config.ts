import { Type } from '@sinclair/typebox'

export const ServerConfigSchema = Type.Object({
  apps: Type.Array(
    Type.Object({
      name: Type.String({ description: 'Unique app name, matches the deploy/:app route param' }),
      compose_path: Type.String({ description: 'Absolute path to the compose file on the VPS' }),
      steps: Type.Array(
        Type.Object({
          service: Type.String({ description: 'Docker Compose service name to roll out' }),
        }),
        { description: 'Ordered rollout steps — executed sequentially' },
      ),
    }),
    { description: 'Registered apps' },
  ),
  notifications: Type.Optional(Type.Object({
    webhook: Type.Optional(Type.String({ description: 'URL to POST job result JSON to' })),
  })),
}, {
  $schema: 'http://json-schema.org/draft-07/schema#',
  title: 'RollHook Server Config',
  description: 'rollhook.config.yaml — server-side configuration',
})
