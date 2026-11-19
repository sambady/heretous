export interface User {
  id: string
  name: string
  is_guest: boolean
}

export interface Session {
  user: User
  expires_at: string
}

export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isSession(value: unknown): value is Session {
  return isRecord(value)
    && typeof value.expires_at === 'string'
    && isRecord(value.user)
    && typeof value.user.id === 'string'
    && typeof value.user.name === 'string'
    && typeof value.user.is_guest === 'boolean'
}

function request(path: string, options: RequestInit & { method: 'DELETE' }): Promise<null>
function request(path: string, options?: RequestInit): Promise<Session>
async function request(path: string, options: RequestInit = {}): Promise<Session | null> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10000)
  try {
    const response = await fetch(path, {
      ...options,
      credentials: 'same-origin',
      cache: 'no-store',
      signal: controller.signal,
    })
    if (response.status === 204 && options.method === 'DELETE') return null
    const body: unknown = await response.json().catch(() => null)
    if (!response.ok) {
      const message = isRecord(body) && isRecord(body.error) && typeof body.error.message === 'string'
        ? body.error.message
        : 'Не удалось выполнить запрос. Попробуй ещё раз.'
      throw new ApiError(message, response.status)
    }
    if (!isSession(body)) throw new ApiError('Не удалось прочитать ответ сервиса.', response.status)
    return body
  } catch (error) {
    if (error instanceof ApiError) throw error
    throw new ApiError('Не удалось связаться с сервером. Проверь соединение и попробуй ещё раз.', 0)
  } finally {
    clearTimeout(timeout)
  }
}

export async function getSession(): Promise<Session | null> {
  try {
    return await request('/api/session')
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) return null
    throw error
  }
}

export function enterAsGuest(name: string): Promise<Session> {
  return request('/api/auth/guest', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
}

export function logout(): Promise<null> {
  return request('/api/session', { method: 'DELETE' })
}
