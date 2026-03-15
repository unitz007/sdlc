export function applyTheme(isDark) {
  const className = 'dark-mode';
  const body = document.body;
  if (isDark) {
    body.classList.add(className);
    // Dynamically load dark.css if not already loaded
    if (!document.getElementById('dark-theme')) {
      const link = document.createElement('link');
      link.id = 'dark-theme';
      link.rel = 'stylesheet';
      link.href = '/src/theme/dark.css';
      document.head.appendChild(link);
    }
  } else {
    body.classList.remove(className);
    const link = document.getElementById('dark-theme');
    if (link) {
      link.remove();
    }
  }
}
