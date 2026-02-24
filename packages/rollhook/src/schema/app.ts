import { Type } from '@sinclair/typebox'

export const AppConfigSchema = Type.Object({
  name: Type.String({ description: 'Unique app name, matches the deploy/:app route param' }),
  compose_file: Type.Optional(Type.String({ description: 'Path to docker-compose file relative to clone_path, defaults to compose.yml', default: 'compose.yml' })),
  steps: Type.Array(
    Type.Object({
      service: Type.String({ description: 'Docker Compose service name to roll out' }),
    }),
    { description: 'Ordered rollout steps — executed sequentially' },
  ),
}, {
  $schema: 'http://json-schema.org/draft-07/schema#',
  title: 'RollHook App Config',
  description: 'Per-app rollhook.yaml configuration',
})
