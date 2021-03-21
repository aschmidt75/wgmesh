//src/main.js
import Vue from 'vue'
import Home from './pages/Home.vue'
import NotFound from './pages/NotFound.vue'
import BootstrapVue from 'bootstrap-vue'
import VueRouter from 'vue-router'
import 'bootstrap/dist/css/bootstrap.css'
import 'bootstrap-vue/dist/bootstrap-vue.css'

Vue.use(BootstrapVue)
Vue.use(VueRouter)
Vue.config.productionTip = false


const routes = {
  '/': Home,
  '/notFound': NotFound,
}

new Vue({
  el: '#app',
  data: {
    currentRoute: window.location.pathname
  },
  computed: {
    ViewComponent () {
      let r = this.currentRoute;
      if (!r.startsWith("/")) {
        r = "/"+r;
      }
      const res = routes[r] || NotFound;
      console.log(res)
      return res
    }
  },
  render (h) { return h(this.ViewComponent) }
}).$mount('#app')


