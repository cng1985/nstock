import { createRouter, createWebHistory } from 'vue-router'
import { session } from './auth'
import Dashboard from './views/Dashboard.vue'
import Market from './views/Market.vue'
import Quote from './views/Quote.vue'
import Predict from './views/Predict.vue'
import Quant from './views/Quant.vue'
import Monitor from './views/Monitor.vue'
import Login from './views/Login.vue'
import Access from './views/Access.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true } },
    { path: '/', component: Dashboard },
    { path: '/market', component: Market, meta: { any: ['stock:quote:view'] } },
    { path: '/quote/:code', component: Quote, meta: { any: ['stock:quote:view'] } },
    { path: '/predict', component: Predict, meta: { any: ['stock:predict:view', 'stock:predict:execute'] } },
    { path: '/quant', component: Quant, meta: { any: ['stock:strategy:view', 'stock:order:view', 'stock:account:view'] } },
    { path: '/monitor', component: Monitor, meta: { any: ['stock:alert:view', 'stock:monitor:view'] } },
    { path: '/access', component: Access, meta: { any: ['stock:user:view', 'stock:role:view'] } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  if (!session.ready) await session.restore()
  if (to.meta.public) {
    if (session.user && to.path === '/login') return '/'
    return true
  }
  if (!session.user) {
    return { path: '/login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } }
  }
  if (to.meta.any && !session.can(...to.meta.any)) return '/'
  return true
})
