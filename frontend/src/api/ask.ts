import { withAuthRetry } from '@/api/auth'
import { apiFetch } from '@/api/http'

export interface AskSource {
  title: string
  url: string
}

export interface AskReply {
  answer: string
  sources: AskSource[]
}

export function askCar(id: string, question: string, signal?: AbortSignal): Promise<AskReply> {
  return withAuthRetry(() =>
    apiFetch<AskReply>(`/vehicles/${encodeURIComponent(id)}/ask`, {
      method: 'POST',
      auth: true,
      body: { question },
      signal,
    }),
  )
}
