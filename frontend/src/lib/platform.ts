/**
 * Wallpapers are a desktop feature: phones and tablets never run any of
 * their logic, show their buttons or download their images. A desktop has
 * a precise pointer that can hover; iPadOS claims to be a Mac but has touch.
 */
export const isDesktop = (() => {
  const precise = window.matchMedia("(hover: hover) and (pointer: fine)").matches;
  const mobileUA = /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent);
  const iPad = /Macintosh/.test(navigator.userAgent) && navigator.maxTouchPoints > 1;
  return precise && !mobileUA && !iPad;
})();
