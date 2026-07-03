package db

// NoteRepo implementa repository.NoteStore.
// Delega para Store (compartilha a mesma conexão e mutex).
type NoteRepo struct {
	store *Store
}

func NewNoteRepo(store *Store) *NoteRepo {
	return &NoteRepo{store: store}
}

func (r *NoteRepo) GetNote(filename string) (string, error) {
	return r.store.GetNote(filename)
}

func (r *NoteRepo) SaveNote(filename, content, mtime string) error {
	return r.store.SaveNote(filename, content, mtime)
}

func (r *NoteRepo) DeleteNote(filename string) error {
	return r.store.DeleteNote(filename)
}

func (r *NoteRepo) RenameNote(old, new string) error {
	return r.store.RenameNote(old, new)
}

func (r *NoteRepo) GetAllNotes() (map[string]string, error) {
	return r.store.GetAllNotes()
}

func (r *NoteRepo) GetNoteMtime(filename string) (string, error) {
	return r.store.GetNoteMtime(filename)
}

func (r *NoteRepo) NoteExists(filename string) bool {
	return r.store.NoteExists(filename)
}
