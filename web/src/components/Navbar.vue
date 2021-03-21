<template>
    <div>
      <b-navbar toggleable="lg" type="dark" variant="dark">
        <b-container>
            <b-navbar-brand :to="{ path: '/' }">wgmesh</b-navbar-brand>
            <b-navbar-toggle target="nav-collapse"></b-navbar-toggle>
            <b-navbar-nav class="ml-auto" align="left">
            </b-navbar-nav>
          </b-container>
      </b-navbar>
    </div>  
</template>
<script>
export default {
    data() {
        return {
            filter: ''
        }
    },
  methods: {
  }
}    
</script>