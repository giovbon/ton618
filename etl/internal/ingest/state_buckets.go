package ingest

// Bucket names for BBolt state database.
// Defined as package-level vars so all state_*.go files can access them.
var (
	bucketFileMods        = []byte("file_mods")
	bucketHashes          = []byte("hashes")
	bucketPopularity      = []byte("popularity")
	bucketKnownTags       = []byte("known_tags")
	bucketFileTags        = []byte("file_tags")
	bucketLinkCounts      = []byte("link_counts")
	bucketFileLinks       = []byte("file_links")
	bucketSettings        = []byte("settings")
	bucketFileMetadata    = []byte("file_metadata")
	bucketVectorHashes    = []byte("vector_hashes")
	bucketNoteVectors     = []byte("note_vectors")
	bucketNoteProjections = []byte("note_projections")
)
