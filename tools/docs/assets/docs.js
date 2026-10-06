'use strict';
const menu = document.querySelector('.menu-button');
menu.addEventListener('click', () => {
  const open = document.body.classList.toggle('menu-open');
  menu.setAttribute('aria-expanded', String(open));
});
const links = [...document.querySelectorAll('.toc a')];
const closeMenu = () => { document.body.classList.remove('menu-open'); menu.setAttribute('aria-expanded', 'false'); };
links.forEach(link => link.addEventListener('click', closeMenu));
document.addEventListener('keydown', event => { if (event.key === 'Escape') closeMenu(); });
document.querySelector('#find-section').addEventListener('input', event => {
  const query = event.target.value.trim().toLocaleLowerCase();
  links.forEach(link => { link.hidden = !link.textContent.toLocaleLowerCase().includes(query); });
  document.querySelector('#no-results').hidden = links.some(link => !link.hidden);
});
document.querySelectorAll('article table').forEach(table => {
  const wrapper = document.createElement('div'); wrapper.className = 'table-wrap';
  table.before(wrapper); wrapper.append(table);
});
document.querySelectorAll('pre').forEach(pre => {
  const code = pre.querySelector('code'); if (!code) return;
  const button = document.createElement('button'); button.className = 'copy-button';
  button.textContent = document.body.dataset.copy; button.setAttribute('aria-live', 'polite');
  button.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(code.textContent);
      button.textContent = document.body.dataset.copied;
    } catch {
      const selection = window.getSelection(); const range = document.createRange();
      range.selectNodeContents(code); selection.removeAllRanges(); selection.addRange(range);
      button.textContent = document.body.dataset.copyFailed;
    }
    setTimeout(() => { button.textContent = document.body.dataset.copy; }, 2200);
  });
  pre.append(button);
});
if ('IntersectionObserver' in window) {
  const observer = new IntersectionObserver(entries => {
    entries.forEach(entry => { if (entry.isIntersecting) links.forEach(link => {
      link.setAttribute('aria-current', String(decodeURIComponent(link.hash).slice(1) === entry.target.id));
    }); });
  }, { rootMargin: '-76px 0px -68% 0px' });
  document.querySelectorAll('article h2, article h3').forEach(heading => observer.observe(heading));
}
