package models

import "time"

type Document struct {
	ID         string   `json:"id"`
	Tipo       string   `json:"tipo"`
	Arquivo    string   `json:"arquivo"`
	Secao      string   `json:"secao"`
	Pagina     int      `json:"pagina"`
	Ordem      int      `json:"ordem"`
	Texto      string   `json:"texto"`
	Timestamp  string   `json:"@timestamp"`
	Created    string   `json:"created_at"`
	Hash       string   `json:"hash"`
	VectorHash string   `json:"vector_hash"`
	Tags       []string `json:"tags,omitempty"`
	IsIndexing bool     `json:"is_indexing"`
	IsNoEmbed  bool     `json:"is_no_embed"`
}

type SearchResults struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []SearchHit `json:"hits"`
	} `json:"hits"`
	SemanticSimilarities map[string]float64 `json:"semantic_similarities,omitempty"`
}

type SearchHit struct {
	ID           string              `json:"_id"`
	Score        float64             `json:"_score"`
	Source       Document            `json:"_source"`
	Highlight    map[string][]string `json:"highlight"`
	FinalScore   float64             `json:"final_score"`
	ScoreDetails map[string]float64  `json:"score_details,omitempty"`
}

type AppSettings struct {
	GoogleVisionKey    string `json:"google_vision_key"`
	SemanticEnable     bool   `json:"semantic_enable"`
	SemanticStrategy   string `json:"semantic_strategy"`   // "whitelist" ou "blacklist"
	Language           string `json:"language"`            // "pt-BR", "en-US"
	EmbeddingDimension int    `json:"embedding_dimension"` // Dimensao do vetor (Matryoshka), default 512
	EmbeddingProvider  string `json:"embedding_provider"`  // "ollama", "openai", "gemini", "custom"
	EmbeddingAPIKey    string `json:"embedding_api_key"`   // Chave da API (Gemini, OpenAI, etc.)
	EmbeddingModel     string `json:"embedding_model"`     // Modelo (ex: "text-embedding-004")
	EmbeddingBaseURL   string `json:"embedding_base_url"`  // URL customizada do endpoint
}

type SystemState struct {
	FileModCache      map[string]time.Time `json:"file_mod_cache"`
	HashCache         map[string]string    `json:"hash_cache"`
	Popularity        map[string]int       `json:"popularity"`
	KnownTags         map[string]bool      `json:"known_tags"`
	FileTags          map[string][]string  `json:"file_tags"`
	LinkCounts        map[string]int       `json:"link_counts"`
	FileLinks         map[string][]string  `json:"file_links"`
	SemanticTopics    map[string]bool      `json:"semantic_topics"`
	FileSemanticLinks map[string][]string  `json:"file_semantic_links"`
	Settings          AppSettings          `json:"settings"`
}
