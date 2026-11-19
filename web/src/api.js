export class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request(path, options = {}) {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10000)
  try {
    const response = await fetch(path, {
      ...options,
      credentials: 'same-origin',
      cache: 'no-store',
      signal: controller.signal,
    })
    if (response.status === 204) return null
    const body = await response.json().catch(() => null)
    if (!response.ok) {
      throw new ApiError(body?.error?.message ?? 'Не удалось выполнить запрос. Попробуй ещё раз.', response.status)
    }
    if (!body?.user) throw new ApiError('Не удалось прочитать ответ сервиса.', response.status)
    return body
  } catch (error) {
    if (error instanceof ApiError) throw error
    throw new ApiError('Не удалось связаться с сервером. Проверь соединение и попробуй ещё раз.', 0)
  } finally {
    clearTimeout(timeout)
  }
}

export async function getSession() {
  try {
    return await request('/api/session')
  } catch (error) {
    if (error.status === 401) return null
    throw error
  }
}

export function enterAsGuest(name) {
  return request('/api/auth/guest', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
}

export function logout() {
  return request('/api/session', { method: 'DELETE' })
}
