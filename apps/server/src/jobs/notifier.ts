import type { JobResult } from 'rollhook'
import { appendFileSync } from 'node:fs'
import process from 'node:process'
import { loadConfig } from '@/config/loader'

export async function notify(job: JobResult, logPath: string): Promise<void> {
  const config = loadConfig()
  const notifications = config.notifications
  const log = (line: string) => appendFileSync(logPath, `${line}\n`)

  const title = job.status === 'success'
    ? `✅ Deployed ${job.app}`
    : `❌ Deployment failed: ${job.app}`
  const message = `Image: ${job.image_tag}\nStatus: ${job.status}${job.error ? `\nError: ${job.error}` : ''}`

  const promises: Promise<void>[] = []

  const pushoverUserKey = process.env.PUSHOVER_USER_KEY
  const pushoverAppToken = process.env.PUSHOVER_APP_TOKEN
  if (pushoverUserKey && pushoverAppToken) {
    promises.push(sendPushover(pushoverUserKey, pushoverAppToken, title, message, log))
  }

  if (notifications?.webhook) {
    promises.push(sendWebhook(notifications.webhook, job, log))
  }

  await Promise.allSettled(promises)
}

async function sendPushover(userKey: string, appToken: string, title: string, message: string, log: (line: string) => void): Promise<void> {
  const res = await fetch('https://api.pushover.net/1/messages.json', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: appToken, user: userKey, title, message }),
  })
  if (!res.ok)
    log(`[notifier] Pushover failed: ${res.status} ${await res.text()}`)
}

async function sendWebhook(url: string, job: JobResult, log: (line: string) => void): Promise<void> {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(job),
  })
  if (!res.ok)
    log(`[notifier] Webhook failed: ${res.status} ${await res.text()}`)
}
