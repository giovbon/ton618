package db

// KeywordRepo implementa repository.KeywordStore.
type KeywordRepo struct {
	store *Store
}

func NewKeywordRepo(store *Store) *KeywordRepo {
	return &KeywordRepo{store: store}
}

func (r *KeywordRepo) SetNoteKeywords(filename string, keywords []string) error {
	return r.store.SetNoteKeywords(filename, keywords)
}

func (r *KeywordRepo) GetNoteKeywords(filename string) ([]string, error) {
	return r.store.GetNoteKeywords(filename)
}
