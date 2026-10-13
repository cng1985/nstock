import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Market from './views/Market.vue'
import Quote from './views/Quote.vue'
import Predict from './views/Predict.vue'
import Quant from './views/Quant.vue'
import Monitor from './views/Monitor.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Dashboard },
    { path: '/market', component: Market },
    { path: '/quote/:code', component: Quote },
    { path: '/predict', component: Predict },
    { path: '/quant', component: Quant },
    { path: '/monitor', component: Monitor },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
