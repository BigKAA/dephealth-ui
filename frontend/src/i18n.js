// Internationalization module
import en from './locales/en.js';
import ru from './locales/ru.js';

const STORAGE_KEY = 'dephealth-lang';
const SUPPORTED_LANGS = { en, ru };
const DEFAULT_LANG = 'en';

let currentLang = DEFAULT_LANG;
let currentDict = en;

/**
 * Initialize i18n: detect language from localStorage or browser settings.
 */
export function initI18n() {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored && SUPPORTED_LANGS[stored]) {
    currentLang = stored;
  } else {
    // Detect from browser
    const browserLang = (navigator.language || '').split('-')[0].toLowerCase();
    currentLang = SUPPORTED_LANGS[browserLang] ? browserLang : DEFAULT_LANG;
  }
  currentDict = SUPPORTED_LANGS[currentLang];
}

/**
 * Translate a key with optional parameter interpolation.
 * @param {string} key - Dot-notation key (e.g. 'sidebar.state')
 * @param {Object} [params] - Interpolation parameters (e.g. {count: 5})
 * @returns {string} Translated string or the key if not found
 */
export function t(key, params) {
  let text = currentDict[key];
  if (text === undefined) {
    // Fallback to English
    text = en[key];
  }
  if (text === undefined) {
    return key;
  }
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), v);
    }
  }
  return text;
}

/**
 * Set the current language, save to localStorage, and dispatch event.
 * @param {string} lang - Language code ('en' or 'ru')
 */
export function setLanguage(lang) {
  if (!SUPPORTED_LANGS[lang]) return;
  currentLang = lang;
  currentDict = SUPPORTED_LANGS[lang];
  localStorage.setItem(STORAGE_KEY, lang);
  window.dispatchEvent(new CustomEvent('language-changed', { detail: lang }));
}

/**
 * Get current language code.
 * @returns {string}
 */
export function getLanguage() {
  return currentLang;
}

/**
 * Update all DOM elements with data-i18n* attributes.
 */
export function updateI18nDom() {
  // data-i18n → textContent
  document.querySelectorAll('[data-i18n]').forEach((el) => {
    const key = el.getAttribute('data-i18n');
    if (key) el.textContent = t(key);
  });

  // data-i18n-title → title attribute
  document.querySelectorAll('[data-i18n-title]').forEach((el) => {
    const key = el.getAttribute('data-i18n-title');
    if (key) el.title = t(key);
  });

  // data-i18n-placeholder → placeholder attribute
  document.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
    const key = el.getAttribute('data-i18n-placeholder');
    if (key) el.placeholder = t(key);
  });

  // data-i18n-html → innerHTML (for elements with embedded HTML like legend items)
  document.querySelectorAll('[data-i18n-html]').forEach((el) => {
    const key = el.getAttribute('data-i18n-html');
    if (key) el.innerHTML = t(key);
  });
}
