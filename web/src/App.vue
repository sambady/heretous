<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { enterAsGuest, getSession, logout, type Session } from './api'

const name = ref('')
const session = ref<Session | null>(null)
const phase = ref<'loading' | 'ready' | 'unavailable'>('loading')
const busy = ref(false)
const error = ref('')
const normalizedName = computed(() => name.value.trim().replace(/\s+/gu, ' '))
const nameLength = computed(() => Array.from(normalizedName.value).length)
const validName = computed(() => nameLength.value >= 1 && nameLength.value <= 32)
let restoring = false
let expiryTimer: ReturnType<typeof setTimeout> | undefined

function errorMessage(failure: unknown): string {
  return failure instanceof Error ? failure.message : 'Не удалось выполнить запрос. Попробуй ещё раз.'
}

function setSession(value: Session | null) {
  session.value = value
  clearTimeout(expiryTimer)
  if (value) {
    // Recheck with the server when the cookie expires, including after sleep.
    const delay = Math.max(1000, new Date(value.expires_at).getTime() - Date.now())
    expiryTimer = setTimeout(restoreSession, Math.min(delay, 2147483647))
  }
}

async function restoreSession() {
  if (busy.value || restoring) return
  restoring = true
  error.value = ''
  try {
    setSession(await getSession())
    phase.value = 'ready'
  } catch (failure) {
    error.value = errorMessage(failure)
    phase.value = 'unavailable'
  } finally {
    restoring = false
  }
}

async function enter() {
  if (!validName.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    setSession(await enterAsGuest(normalizedName.value))
    name.value = ''
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    busy.value = false
  }
}

async function leave() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await logout()
    setSession(null)
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    busy.value = false
  }
}

function onVisibilityChange() {
  if (document.visibilityState === 'visible') restoreSession()
}

onMounted(() => {
  restoreSession()
  document.addEventListener('visibilitychange', onVisibilityChange)
})
onUnmounted(() => {
  clearTimeout(expiryTimer)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<template>
  <main class="app" :aria-busy="phase === 'loading' || busy">
    <p v-if="phase === 'loading'" role="status">Загрузка…</p>

    <div v-else-if="phase === 'unavailable'" class="form">
      <p class="error-message" role="alert">{{ error }}</p>
      <button type="button" @click="restoreSession">Повторить</button>
    </div>

    <div v-else-if="session" class="form">
      <p>{{ session.user.name }}</p>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
      <button type="button" :disabled="busy" @click="leave">{{ busy ? 'Выход…' : 'Выйти' }}</button>
    </div>

    <form v-else class="form" @submit.prevent="enter">
      <label for="guest-name">Имя</label>
      <input
        id="guest-name"
        v-model="name"
        name="name"
        type="text"
        autocomplete="nickname"
        required
        maxlength="128"
        :disabled="busy"
        :aria-invalid="nameLength > 32"
        :aria-describedby="nameLength > 32 || error ? 'name-error' : undefined"
        @input="error = ''"
      />
      <p v-if="nameLength > 32" id="name-error" class="error-message" role="alert">Не больше 32 символов.</p>
      <p v-else-if="error" id="name-error" class="error-message" role="alert">{{ error }}</p>
      <button type="submit" :disabled="busy || !validName">{{ busy ? 'Вход…' : 'Войти' }}</button>
    </form>
  </main>
</template>
