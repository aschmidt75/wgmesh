<template>
  <div id="app">
    <Navbar />
    <div id="maincontainer">
          <NodesTable />
    </div>
  </div>
</template>

<script>
import Navbar from '../components/Navbar.vue'
import NodesTable from '../components/NodesTable.vue';

export default {
  name: 'Home',
  components: {
    Navbar,
    NodesTable,
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
