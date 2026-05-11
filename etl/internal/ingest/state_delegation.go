package ingest

// -- LinkCounts (delegacao para LinkManager) --

func (s *AppState) UpdateLinkCounts(newCounts map[string]int) { s.links.UpdateLinkCounts(newCounts) }
func (s *AppState) GetLinkCount(filename string) int          { return s.links.GetLinkCount(filename) }

// -- FileLinks (delegacao para LinkManager) --

func (s *AppState) SetFileLinks(filename string, links []string) {
	s.links.SetFileLinks(filename, links)
}
func (s *AppState) GetFileLinks(filename string) []string { return s.links.GetFileLinks(filename) }
func (s *AppState) GetAllFileLinks() map[string][]string  { return s.links.GetAllFileLinks() }
func (s *AppState) DeleteFileLinks(filename string)       { s.links.DeleteFileLinks(filename) }

// -- Tags (delegacao para TagManager) --

func (s *AppState) AddKnownTag(tag string)                     { s.tags.AddKnownTag(tag) }
func (s *AppState) GetAllKnownTags() []string                  { return s.tags.GetAllKnownTags() }
func (s *AppState) GetKnownTagsCount() int                     { return s.tags.GetKnownTagsCount() }
func (s *AppState) SetFileTags(filename string, tags []string) { s.tags.SetFileTags(filename, tags) }
func (s *AppState) GetFileTags(filename string) []string       { return s.tags.GetFileTags(filename) }
func (s *AppState) HasTags(filename string) bool               { return s.tags.HasTags(filename) }
func (s *AppState) DeleteFileTags(filename string)             { s.tags.DeleteFileTags(filename) }

// -- Vector Hashes (delegacao para VectorManager) --

func (s *AppState) GetVectorHash(id string) (string, bool) { return s.vectors.GetVectorHash(id) }
func (s *AppState) GetVectorHashCount() int                { return s.vectors.GetVectorHashCount() }
func (s *AppState) SetVectorHash(id, hash string)          { s.vectors.SetVectorHash(id, hash) }
func (s *AppState) DeleteVectorHash(id string)             { s.vectors.DeleteVectorHash(id) }

// -- Note Vectors (delegacao para VectorManager) --

func (s *AppState) SetNoteVector(id string, vector []float32, title string) {
	s.vectors.SetNoteVector(id, vector, title)
}
func (s *AppState) GetAllNoteVectors() map[string]NoteVector { return s.vectors.GetAllNoteVectors() }
func (s *AppState) ClearNoteVectors() error                  { return s.vectors.ClearNoteVectors() }

// -- Note Projections (delegacao para VectorManager) --

func (s *AppState) SetNoteProjections(projections map[string][]float64) {
	s.vectors.SetNoteProjections(projections)
}
func (s *AppState) GetAllNoteProjections() map[string][]float64 {
	return s.vectors.GetAllNoteProjections()
}
func (s *AppState) ClearNoteProjections() error              { return s.vectors.ClearNoteProjections() }
func (s *AppState) DeleteNoteProjection(id string)           { s.vectors.DeleteNoteProjection(id) }
func (s *AppState) SetNoteVectors2D(id string, x, y float64) { s.vectors.SetNoteVectors2D(id, x, y) }

// -- Semantic Links (delegacao para SemanticManager) --

func (s *AppState) SetFileSemanticLinks(filename string, links []string) {
	s.semantic.SetFileSemanticLinks(filename, links)
}
func (s *AppState) GetFileSemanticLinks(filename string) []string {
	return s.semantic.GetFileSemanticLinks(filename)
}
func (s *AppState) GetAllFileSemanticLinks() map[string][]string {
	return s.semantic.GetAllFileSemanticLinks()
}
func (s *AppState) GetAllSemanticTopics() []string {
	return s.semantic.GetAllSemanticTopics()
}
func (s *AppState) DeleteFileSemanticLinks(filename string) {
	s.semantic.DeleteFileSemanticLinks(filename)
}
func (s *AppState) RebuildSemanticTopics() {
	s.semantic.RebuildSemanticTopics()
}
