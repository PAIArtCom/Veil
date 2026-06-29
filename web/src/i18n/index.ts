import en from './en';
import zh from './zh';
import ja from './ja';
import es from './es';

export type Locale = 'en' | 'zh' | 'ja' | 'es';
export type Translations = typeof en;

const translations: Record<Locale, Translations> = { en, zh, ja, es };

export const locales: Locale[] = ['en', 'zh', 'ja', 'es'];

export const localeLabels: Record<Locale, string> = {
  en: 'EN',
  zh: '中文',
  ja: '日本語',
  es: 'ES',
};

export function useTranslations(locale: string | undefined): Translations {
  const key = (locale ?? 'en') as Locale;
  return translations[key] ?? translations.en;
}

export function getLocalePath(locale: string | undefined, path: string = '/'): string {
  if (!locale || locale === 'en') return path;
  return `/${locale}${path}`;
}
