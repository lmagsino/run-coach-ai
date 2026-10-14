import { createRouter, createWebHistory } from 'vue-router'
import { useSession } from './lib/useSession.js'
import ChatPage from './pages/ChatPage.vue'
import ConnectionPage from './pages/ConnectionPage.vue'
import LoginPage from './pages/LoginPage.vue'
import SignUpPage from './pages/SignUpPage.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginPage, meta: { public: true } },
    { path: '/signup', name: 'signup', component: SignUpPage, meta: { public: true } },
    { path: '/', name: 'home', component: ChatPage },
    { path: '/c/:id', name: 'chat', component: ChatPage, props: true },
    { path: '/connection', name: 'connection', component: ConnectionPage },
    { path: '/:rest(.*)', redirect: '/' },
  ],
})

// Every page needs a session except login. The first navigation loads /me;
// a 401 sends the visitor to the login page. Mock mode answers /me without a
// cookie, so nothing redirects there.
router.beforeEach(async (to) => {
  const session = useSession()
  if (!session.loaded.value) {
    try {
      await session.load()
    } catch {
      // The backend may be asleep or down; the login page shows the waking
      // state and the chat page shows the error when it tries to send.
    }
  }
  if (to.meta.public) return true
  if (session.unauthorized.value) return { name: 'login', query: to.fullPath !== '/' ? { next: to.fullPath } : {} }
  return true
})
