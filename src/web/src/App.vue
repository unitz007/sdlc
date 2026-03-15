<template>
  <div id="app">
    <header>
      <h1>Dashboard</h1>
      <ThemeSwitcher />
    </header>
    <main>
      <!-- Main content -->
    </main>
  </div>
</template>

<script>
import ThemeSwitcher from './components/ThemeSwitcher.vue';
export default {
  name: 'App',
  components: { ThemeSwitcher }
};
</script>

<style src="./index.css"></style>
