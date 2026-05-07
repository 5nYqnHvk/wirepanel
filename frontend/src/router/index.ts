import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const Login = () => import('@/views/Login.vue')
const Layout = () => import('@/views/Layout.vue')
const Dashboard = () => import('@/views/Dashboard.vue')
const Agents = () => import('@/views/Agents.vue')
const AgentDetail = () => import('@/views/AgentDetail.vue')
const Audit = () => import('@/views/Audit.vue')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true } },
    {
      path: '/',
      component: Layout,
      children: [
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', component: Dashboard },
        { path: 'agents', component: Agents },
        { path: 'agents/:id', component: AgentDetail, props: true },
        { path: 'audit', component: Audit }
      ]
    }
  ]
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.authed) return '/login'
  if (to.path === '/login' && auth.authed) return '/dashboard'
  return true
})

export default router
