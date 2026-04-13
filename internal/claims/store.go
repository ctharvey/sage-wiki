package claims

import (
	"database/sql"

	"github.com/xoai/sage-wiki/internal/storage"
)

// Claim represents an extracted claim from wiki content.
type Claim struct {
	ID          string // "claim:<conceptID>:<index>"
	ConceptID   string
	Section     string // "key_claims" or "evidence"
	Text        string // the claim text
	SourcePath  string // path from [Source: ...] marker
	SourceQuote string // optional verbatim quote if present
}

// Store manages claims in SQLite.
type Store struct {
	db *storage.DB
}

// NewStore creates a new claims store.
func NewStore(db *storage.DB) *Store {
	return &Store{db: db}
}

// Init creates the claims table and index if they don't exist.
func (s *Store) Init() error {
	return s.db.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS claims (
				id TEXT PRIMARY KEY,
				concept_id TEXT NOT NULL,
				section TEXT NOT NULL,
				text TEXT NOT NULL,
				source_path TEXT,
				source_quote TEXT
			)
		`)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			CREATE INDEX IF NOT EXISTS claims_concept_idx ON claims(concept_id)
		`)
		return err
	})
}

// Add inserts or replaces a claim in the database.
func (s *Store) Add(c Claim) error {
	return s.db.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			"INSERT OR REPLACE INTO claims (id, concept_id, section, text, source_path, source_quote) VALUES (?, ?, ?, ?, ?, ?)",
			c.ID, c.ConceptID, c.Section, c.Text, c.SourcePath, c.SourceQuote,
		)
		return err
	})
}

// GetByConcept retrieves all claims for a given concept ID.
func (s *Store) GetByConcept(conceptID string) ([]Claim, error) {
	rows, err := s.db.ReadDB().Query(
		"SELECT id, concept_id, section, text, source_path, source_quote FROM claims WHERE concept_id = ?",
		conceptID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var claims []Claim
	for rows.Next() {
		var c Claim
		if err := rows.Scan(&c.ID, &c.ConceptID, &c.Section, &c.Text, &c.SourcePath, &c.SourceQuote); err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

// DeleteByConcept removes all claims for a given concept ID.
func (s *Store) DeleteByConcept(conceptID string) error {
	return s.db.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("DELETE FROM claims WHERE concept_id = ?", conceptID)
		return err
	})
}

// Count returns the total number of claims.
func (s *Store) Count() (int, error) {
	var count int
	err := s.db.ReadDB().QueryRow("SELECT COUNT(*) FROM claims").Scan(&count)
	return count, err
}
