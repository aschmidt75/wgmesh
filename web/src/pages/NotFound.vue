<template>
  <div id="app">
    <Navbar />
    <h3>
        Not found
    </h3>
  </div>
</template>

<script>
import Navbar from '../components/Navbar.vue'

export default {
  name: 'NotFound',
  components: {
    Navbar,
  }
}
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  color: #2c3e50;
  margin-top: 10px;
}
</style>
