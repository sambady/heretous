<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { enterAsGuest, getSession, logout } from './api.js'

const name = ref('')
const session = ref(null)
const phase = ref('loading')
const busy = ref(false)
const error = ref('')
const normalizedName = computed(() => name.value.trim().replace(/\s+/gu, ' '))
const nameLength = computed(() => Array.from(normalizedName.value).length)
const validName = computed(() => nameLength.value >= 1 && nameLength.value <= 32)
let restoring = false
let expiryTimer

function setSession(value) {
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
    error.value = failure.message
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
    error.value = failure.message
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
    error.value = failure.message
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
  <div class="page-shell">
    <header class="site-header">
      <a class="brand" href="/" aria-label="Heretous — главная">
        <img src="/favicon.svg" width="36" height="36" alt="" />
        <span>heretous<span class="brand-dot">.</span></span>
      </a>
      <span class="header-note">Место для живого общения</span>
    </header>

    <main class="entry-layout">
      <section class="introduction" aria-labelledby="intro-title">
        <p class="eyebrow"><span></span> ВСЁ НАЧИНАЕТСЯ С «ПРИВЕТ»</p>
        <h1 id="intro-title">Давай<br /><em>знакомиться.</em></h1>
        <p class="intro-copy">Новые лица, простые разговоры<br class="desktop-break" /> и немного хорошей компании.</p>

        <svg class="table-illustration" viewBox="0 0 440 240" fill="none" aria-hidden="true">
          <ellipse cx="226" cy="206" rx="157" ry="17" fill="#e8e0d1" />
          <path d="M144 158l-13 46m170-46 13 46" stroke="#7a4c39" stroke-width="9" stroke-linecap="round" />
          <path d="M85 101c0-20 14-35 32-35s32 15 32 35v30H85z" fill="#8c9b7d" />
          <circle cx="117" cy="53" r="20" fill="#d7a377" />
          <path d="M97 49c1-26 37-28 40 1-12-2-20-8-23-13-3 8-10 12-17 12z" fill="#49332e" />
          <path d="M299 98c0-21 14-35 32-35s32 14 32 35v31h-64z" fill="#cc765d" />
          <circle cx="331" cy="50" r="20" fill="#f0c6a0" />
          <path d="M312 50c-11-33 47-40 41 8l-6 12-3-28c-13 2-22-3-25-7z" fill="#684438" />
          <path d="M188 75c0-19 14-32 32-32s32 13 32 32v28h-64z" fill="#d8b467" />
          <circle cx="220" cy="29" r="19" fill="#b77b56" />
          <path d="M202 25c-3-22 34-25 36 0-14 1-22-6-25-10-3 6-5 8-11 10z" fill="#342a28" />
          <ellipse cx="224" cy="133" rx="148" ry="52" fill="#b87952" />
          <ellipse cx="224" cy="126" rx="148" ry="51" fill="#deb88a" />
          <ellipse cx="224" cy="126" rx="136" ry="42" stroke="#c99c6b" stroke-dasharray="3 6" />
          <path d="M141 108h19l-2 22h-15z" fill="#f9f0dc" />
          <path d="M161 113c12-1 12 13-2 12" stroke="#f9f0dc" stroke-width="4" />
          <path d="M282 112h16l-2 24h-12z" fill="#faf3df" />
          <path d="M285 124h11l-1 10h-9z" fill="#ce8053" />
          <path d="M144 97c-6-5 5-7 0-13m9 12c-5-6 5-9 1-14" stroke="#a27b60" stroke-width="2" stroke-linecap="round" />
          <ellipse cx="219" cy="116" rx="18" ry="7" fill="#fbf1d7" />
          <path d="M211 114l7-7 10 7-8 5z" fill="#c7824c" />
          <path d="M377 53l12-9m-6 24 16-1M65 47l-11-9m4 24-16-2" stroke="#c98568" stroke-width="3" stroke-linecap="round" />
        </svg>
        <span class="illustration-caption">За хорошим разговором всегда найдётся место.</span>
      </section>

      <section class="entry-card" aria-labelledby="card-title" :aria-busy="phase === 'loading' || busy">
        <div v-if="phase === 'loading'" class="status-panel" role="status">
          <span class="loading-dot"></span>
          <h2 id="card-title">Секунду…</h2>
          <p>Рады тебя видеть.</p>
        </div>

        <div v-else-if="phase === 'unavailable'" class="status-panel">
          <p class="card-kicker">ЧТО-ТО ПОШЛО НЕ ТАК</p>
          <h2 id="card-title">Не удалось войти</h2>
          <p class="error-message" role="alert">{{ error }}</p>
          <button class="primary-button" type="button" @click="restoreSession">Попробовать ещё раз <span aria-hidden="true">↻</span></button>
        </div>

        <div v-else-if="session" class="welcome-panel">
          <div class="welcome-mark" aria-hidden="true">✳</div>
          <span class="guest-badge">В гостях</span>
          <h2 id="card-title" class="welcome-title">Привет,<br />{{ session.user.name }}!</h2>
          <p class="card-description">Ты вошёл как гость.<br />Теперь мы знакомы. Располагайся!</p>
          <p v-if="error" class="error-message" role="alert">{{ error }}</p>
          <button class="secondary-button" type="button" :disabled="busy" @click="leave">{{ busy ? 'Выходим…' : 'Выйти' }}</button>
        </div>

        <template v-else>
          <div class="card-heading">
            <p class="card-kicker">ЗАХОДИ, МЫ РАДЫ</p>
            <h2 id="card-title">Как тебя называть?</h2>
            <p class="card-description">Для начала достаточно имени.</p>
          </div>
          <form class="entry-form" @submit.prevent="enter">
            <label for="guest-name">Твоё имя</label>
            <input id="guest-name" v-model="name" name="name" type="text" autocomplete="nickname" placeholder="Например, Саша" required maxlength="128" :disabled="busy" :aria-invalid="nameLength > 32" :aria-describedby="nameLength > 32 || error ? 'name-hint name-error' : 'name-hint'" @input="error = ''" />
            <p id="name-hint" class="field-hint">От 1 до 32 символов. Имя увидят собеседники.</p>
            <p v-if="nameLength > 32" id="name-error" class="error-message" role="alert">Имя должно быть не длиннее 32 символов.</p>
            <p v-else-if="error" id="name-error" class="error-message" role="alert">{{ error }}</p>
            <button class="primary-button" type="submit" :disabled="busy || !validName">{{ busy ? 'Заходим…' : 'Войти как гость' }}<span aria-hidden="true">↗</span></button>
          </form>
          <div class="card-footnote"><span aria-hidden="true">✧</span><p>Никаких паролей<br />и лишних вопросов.</p></div>
        </template>
      </section>
    </main>

    <footer class="site-footer"><span>Будь собой. Будь в компании.</span><span>heretous · живые разговоры</span></footer>
  </div>
</template>
