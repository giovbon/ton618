package db

// LinkRepo implementa repository.LinkStore.
type LinkRepo struct {
	store *Store
}

func NewLinkRepo(store *Store) *LinkRepo {
	return &LinkRepo{store: store}
}

func (r *LinkRepo) AddLink(fromFile, toFile string) error {
	return r.store.AddLink(fromFile, toFile)
}

func (r *LinkRepo) ClearLinks(fromFile string) error {
	return r.store.ClearLinks(fromFile)
}

func (r *LinkRepo) GetBacklinks(toFile string) ([]string, error) {
	return r.store.GetBacklinks(toFile)
}

func (r *LinkRepo) GetLinks(fromFile string) ([]string, error) {
	return r.store.GetLinks(fromFile)
}

func (r *LinkRepo) GetLinksByFiles(fromFiles []string, exclude map[string]bool) ([]string, error) {
	return r.store.GetLinksByFiles(fromFiles, exclude)
}
