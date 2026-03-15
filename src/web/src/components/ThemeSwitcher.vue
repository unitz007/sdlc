<template>
  <button @click="toggleTheme">Dark Mode</button>
</template>
<script>
import { applyTheme } from '../theme';
export default {
  name: 'ThemeSwitcher',
  data() {
    return {
      isDark: localStorage.getItem('theme') === 'dark'
    };
  },
  mounted() {
    applyTheme(this.isDark);
  },
  methods: {
    toggleTheme() {
      this.isDark = !this.isDark;
      localStorage.setItem('theme', this.isDark ? 'dark' : 'light');
      applyTheme(this.isDark);
    }
  }
};
</script>
