export interface FileObject {
  name: string;
  content: string;
  isNew?: boolean;
  scrollToText?: string | null;
}

export interface AppSettings {
  semantic_enable: boolean;
  semantic_threshold: number;
  google_vision_key?: string;
  semantic_strategy?: string;
  ollama_hosts?: string[];
  ollama_host_active?: string;
}

export interface Toast {
  id: number;
  message: string;
  type: 'info' | 'success' | 'error' | 'warning';
}

export interface RankingWeights {
  base_multiplier: number;
  semantic_alpha: number;
  boost_title_exact: number;
  boost_title_partial: number;
  boost_path_context: number;
  boost_phrase: number;
  boost_freshness_max: number;
  boost_semantic: number;
  boost_technical: number;
  boost_link_authority: number;
}

export interface TagAutocompleteState {
  active: boolean;
  matches: string[];
  selectedIndex: number;
  queryText: string;
}

export interface LastEditedFile {
  name: string | null;
  ts: string | null;
}

export interface SearchResult {
  id: string;
  arquivo: string;
  texto: string;
  tipo: 'note' | 'image' | 'imagem' | 'pdf' | 'link' | 'desenho' | 'markdown';
  tags?: string[];
  score_details?: Record<string, number>;
  final_score?: number;
  pagina?: number;
  secao?: string;
  '@timestamp'?: string;
  is_indexing?: boolean;
  _source?: any;
}
