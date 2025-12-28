package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"

	"github.com/example/linkedbot/internal/models"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() { _ = s.db.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	stmt := `
CREATE TABLE IF NOT EXISTS profiles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	linkedin_url TEXT NOT NULL UNIQUE,
	name TEXT,
	headline TEXT,
	company TEXT,
	location TEXT,
	connection_sent INTEGER DEFAULT 0,
	connection_sent_at DATETIME,
	connection_accepted INTEGER DEFAULT 0,
	connection_checked_at DATETIME,
	message_sent INTEGER DEFAULT 0,
	message_sent_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS message_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id INTEGER NOT NULL,
	type TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	FOREIGN KEY(profile_id) REFERENCES profiles(id)
);
CREATE TABLE IF NOT EXISTS run_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_type TEXT NOT NULL,
	started_at DATETIME NOT NULL,
	ended_at DATETIME NOT NULL,
	summary TEXT
);
`
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

func (s *Store) UpsertProfile(ctx context.Context, p *models.Profile) (int64, error) {
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	res, err := s.db.ExecContext(ctx, `INSERT INTO profiles (linkedin_url, name, headline, company, location, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(linkedin_url) DO UPDATE SET
		name=excluded.name,
		headline=excluded.headline,
		company=excluded.company,
		location=excluded.location,
		updated_at=excluded.updated_at
	`, p.LinkedInURL, p.Name, p.Headline, p.Company, p.Location, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		// fetch existing id
		row := s.db.QueryRowContext(ctx, `SELECT id FROM profiles WHERE linkedin_url = ?`, p.LinkedInURL)
		_ = row.Scan(&id)
	}
	return id, nil
}

func (s *Store) GetProfilesNeedingConnection(ctx context.Context, limit int) ([]models.Profile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, linkedin_url, name, headline, company, location FROM profiles WHERE connection_sent = 0 ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Profile
	for rows.Next() {
		var p models.Profile
		if err := rows.Scan(&p.ID, &p.LinkedInURL, &p.Name, &p.Headline, &p.Company, &p.Location); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) MarkConnectionSent(ctx context.Context, id int64, note string) error {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE profiles SET connection_sent = 1, connection_sent_at = ?, updated_at = ? WHERE id = ?`, now, now, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO message_logs (profile_id, type, content, created_at) VALUES (?, ?, ?, ?)`, id, string(models.MessageTypeConnectionNote), note, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetProfilesNeedingFollowUp(ctx context.Context, limit int) ([]models.Profile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, linkedin_url, name, headline, company, location FROM profiles WHERE connection_sent = 1 AND connection_accepted = 1 AND message_sent = 0 ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Profile
	for rows.Next() {
		var p models.Profile
		if err := rows.Scan(&p.ID, &p.LinkedInURL, &p.Name, &p.Headline, &p.Company, &p.Location); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) MarkMessageSent(ctx context.Context, id int64, content string) error {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE profiles SET message_sent = 1, message_sent_at = ?, updated_at = ? WHERE id = ?`, now, now, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO message_logs (profile_id, type, content, created_at) VALUES (?, ?, ?, ?)`, id, string(models.MessageTypeFollowUp), content, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetPendingAcceptanceChecks(ctx context.Context, limit int) ([]models.Profile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, linkedin_url FROM profiles WHERE connection_sent = 1 AND connection_accepted = 0 ORDER BY connection_sent_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Profile
	for rows.Next() {
		var p models.Profile
		if err := rows.Scan(&p.ID, &p.LinkedInURL); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) MarkAccepted(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `UPDATE profiles SET connection_accepted = 1, connection_checked_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	return err
}

func (s *Store) CountActionsToday(ctx context.Context, table, typeFilter string) (int, error) {
	var row *sql.Row
	if table == "message_logs" && typeFilter != "" {
		row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_logs WHERE type = ? AND DATE(created_at) = DATE('now', 'localtime')`, typeFilter)
	} else if table == "message_logs" {
		row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_logs WHERE DATE(created_at) = DATE('now', 'localtime')`)
	} else if table == "profiles" {
		row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles WHERE connection_sent = 1 AND DATE(connection_sent_at) = DATE('now', 'localtime')`)
	} else {
		return 0, errors.New("unsupported table for CountActionsToday")
	}
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}
