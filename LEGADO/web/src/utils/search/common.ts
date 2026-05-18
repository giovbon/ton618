/**
 * STOPWORDS - Palavras que não agregam valor à busca e são ignoradas
 * para evitar ruído nos resultados.
 */
export const STOPWORDS: string[] = [
  'o',
  'a',
  'os',
  'as',
  'um',
  'uma',
  'uns',
  'umas',
  'de',
  'do',
  'da',
  'dos',
  'das',
  'no',
  'na',
  'nos',
  'nas',
  'ao',
  'aos',
  'à',
  'às',
  'pelo',
  'pela',
  'pelos',
  'pelas',
  'em',
  'num',
  'numa',
  'nuns',
  'numas',
  'com',
  'sem',
  'por',
  'para',
  'ante',
  'até',
  'após',
  'desde',
  'entre',
  'sob',
  'sobre',
  'trás',
  'que',
  'se',
  'e',
  'ou',
  'mas',
  'quem',
  'qual',
  'quais',
  'quanto',
  'quantos',
  'quanta',
  'quantas',
  'como',
  'onde',
  'quando',
  'porque',
  'porquê',
  'vortex',
  'ton',
  'ton-618',
  'ton618',
];

/**
 * REGEX para captura de metadados na query.
 */
export const TAG_REGEX: RegExp = /#([\w\-À-ÿ]+)/g;
export const QUOTE_REGEX: RegExp = /"([^"]+)"/g;
export const SYS_FILTER_REGEX: RegExp = /(?:\+?)(arquivo|secao|tipo|tags):([\w\-./À-ÿ*]+)/g;

/**
 * Verifica se um termo é uma stopword.
 */
export const isStopword = (term: string): boolean => {
  return STOPWORDS.includes(term.toLowerCase()) || term.length < 4;
};
