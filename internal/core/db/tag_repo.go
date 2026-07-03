package db

// TagRepo implementa repository.TagStore.
type TagRepo struct {
	store *Store
}

func NewTagRepo(store *Store) *TagRepo {
	return &TagRepo{store: store}
}

func (r *TagRepo) SetFileTags(arquivo string, tags []string) error {
	return r.store.SetFileTags(arquivo, tags)
}

func (r *TagRepo) GetFileTags(arquivo string) ([]string, error) {
	return r.store.GetFileTags(arquivo)
}

func (r *TagRepo) GetAllTags() ([]string, error) {
	return r.store.GetAllTags()
}
