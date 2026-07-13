package db

// FileModRepo implementa repository.FileModStore.
type FileModRepo struct {
	store *Store
}

func NewFileModRepo(store *Store) *FileModRepo {
	return &FileModRepo{store: store}
}

func (r *FileModRepo) GetFileMod(arquivo string) (string, error) {
	return r.store.GetFileMod(arquivo)
}

func (r *FileModRepo) SetFileMod(arquivo, mtime string) error {
	return r.store.SetFileMod(arquivo, mtime)
}

func (r *FileModRepo) DeleteFileMod(arquivo string) error {
	return r.store.DeleteFileMod(arquivo)
}

func (r *FileModRepo) GetAllFileMods() (map[string]string, error) {
	return r.store.GetAllFileMods()
}
