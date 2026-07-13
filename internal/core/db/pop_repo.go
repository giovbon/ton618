package db

// PopRepo implementa repository.PopStore.
type PopRepo struct {
	store *Store
}

func NewPopRepo(store *Store) *PopRepo {
	return &PopRepo{store: store}
}

func (r *PopRepo) GetPopularity(arquivo string) int {
	return r.store.GetPopularity(arquivo)
}

func (r *PopRepo) IncrementPopularity(arquivo string) error {
	return r.store.IncrementPopularity(arquivo)
}

func (r *PopRepo) ResetPopularity(arquivo string) error {
	return r.store.ResetPopularity(arquivo)
}
