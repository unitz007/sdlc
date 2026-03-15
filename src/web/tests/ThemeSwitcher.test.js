import { mount } from '@vue/test-utils';
import ThemeSwitcher from '../../src/components/ThemeSwitcher.vue';

test('toggles dark mode and persists', async () => {
  const wrapper = mount(ThemeSwitcher);
  // initial state based on localStorage (none -> false)
  expect(wrapper.vm.isDark).toBe(false);
  await wrapper.find('button').trigger('click');
  expect(wrapper.vm.isDark).toBe(true);
  expect(localStorage.getItem('theme')).toBe('dark');
});
