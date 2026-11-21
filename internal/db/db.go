// Package db owns the SQLite database: schema, migrations and queries.
package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type DB struct{ *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS providers (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	kind       TEXT NOT NULL,          -- openai | anthropic
	base_url   TEXT NOT NULL,
	api_key    TEXT NOT NULL DEFAULT '',
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chats (
	id            TEXT PRIMARY KEY,
	title         TEXT NOT NULL DEFAULT 'New chat',
	system_prompt TEXT NOT NULL DEFAULT '',
	model         TEXT NOT NULL DEFAULT '',   -- "<provider_id>/<model_id>"
	leaf_id       TEXT NOT NULL DEFAULT '',   -- tip of the active branch
	pinned        INTEGER NOT NULL DEFAULT 0,
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS chats_updated ON chats(updated_at DESC);

CREATE TABLE IF NOT EXISTS messages (
	id         TEXT PRIMARY KEY,
	chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
	parent_id  TEXT NOT NULL DEFAULT '',
	role       TEXT NOT NULL,           -- user | assistant
	content    TEXT NOT NULL DEFAULT '',
	images     TEXT NOT NULL DEFAULT '[]', -- JSON array of data URLs
	model      TEXT NOT NULL DEFAULT '',
	error      TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS messages_chat ON messages(chat_id, created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
	content, chat_id UNINDEXED, content='messages', content_rowid='rowid'
);
CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
	INSERT INTO messages_fts(rowid, content, chat_id) VALUES (new.rowid, new.content, new.chat_id);
END;
CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
	INSERT INTO messages_fts(messages_fts, rowid, content, chat_id) VALUES ('delete', old.rowid, old.content, old.chat_id);
END;
CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE OF content ON messages BEGIN
	INSERT INTO messages_fts(messages_fts, rowid, content, chat_id) VALUES ('delete', old.rowid, old.content, old.chat_id);
	INSERT INTO messages_fts(rowid, content, chat_id) VALUES (new.rowid, new.content, new.chat_id);
END;

CREATE TABLE IF NOT EXISTS presets (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	prompt     TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
`

func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", path)
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sdb.SetMaxOpenConns(1) // SQLite: one writer; keeps things simple and race-free
	if _, err := sdb.Exec(schema); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	d := &DB{sdb}
	if err := d.migrateExtra(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func NewID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	// time prefix keeps ids roughly sortable
	return fmt.Sprintf("%x%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

func now() int64 { return time.Now().UnixMilli() }

// ---------- settings ----------

func (d *DB) GetSetting(key string) (string, error) {
	var v string
	err := d.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return v, err
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// ---------- sessions ----------

func (d *DB) CreateSession(ttl time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	_, err := d.Exec(`INSERT INTO sessions(token,expires_at) VALUES(?,?)`, tok, time.Now().Add(ttl).UnixMilli())
	return tok, err
}

func (d *DB) ValidSession(tok string) bool {
	var exp int64
	if err := d.QueryRow(`SELECT expires_at FROM sessions WHERE token=?`, tok).Scan(&exp); err != nil {
		return false
	}
	return exp > now()
}

func (d *DB) DeleteSession(tok string) { _, _ = d.Exec(`DELETE FROM sessions WHERE token=?`, tok) }

func (d *DB) DeleteAllSessions() { _, _ = d.Exec(`DELETE FROM sessions`) }

func (d *DB) PruneSessions() { _, _ = d.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now()) }

// ---------- providers ----------

type Provider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"-"`
	HasKey    bool   `json:"has_key"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
}

func scanProvider(s interface{ Scan(...any) error }) (Provider, error) {
	var p Provider
	err := s.Scan(&p.ID, &p.Name, &p.Kind, &p.BaseURL, &p.APIKey, &p.Enabled, &p.CreatedAt)
	p.HasKey = p.APIKey != ""
	return p, err
}

const providerCols = `id,name,kind,base_url,api_key,enabled,created_at`

func (d *DB) ListProviders() ([]Provider, error) {
	rows, err := d.Query(`SELECT ` + providerCols + ` FROM providers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Provider{}
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) GetProvider(id string) (Provider, error) {
	p, err := scanProvider(d.QueryRow(`SELECT `+providerCols+` FROM providers WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (d *DB) SaveProvider(p *Provider) error {
	if p.ID == "" {
		p.ID = NewID()
		p.CreatedAt = now()
		_, err := d.Exec(`INSERT INTO providers(`+providerCols+`) VALUES(?,?,?,?,?,?,?)`,
			p.ID, p.Name, p.Kind, p.BaseURL, p.APIKey, p.Enabled, p.CreatedAt)
		return err
	}
	_, err := d.Exec(`UPDATE providers SET name=?,kind=?,base_url=?,api_key=?,enabled=? WHERE id=?`,
		p.Name, p.Kind, p.BaseURL, p.APIKey, p.Enabled, p.ID)
	return err
}

func (d *DB) DeleteProvider(id string) error {
	_, err := d.Exec(`DELETE FROM providers WHERE id=?`, id)
	return err
}

// ---------- chats ----------

type Chat struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	SystemPrompt string `json:"system_prompt"`
	Model        string `json:"model"`
	LeafID       string `json:"leaf_id"`
	Pinned       bool   `json:"pinned"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

const chatCols = `id,title,system_prompt,model,leaf_id,pinned,created_at,updated_at`

func scanChat(s interface{ Scan(...any) error }) (Chat, error) {
	var c Chat
	err := s.Scan(&c.ID, &c.Title, &c.SystemPrompt, &c.Model, &c.LeafID, &c.Pinned, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (d *DB) ListChats() ([]Chat, error) {
	rows, err := d.Query(`SELECT ` + chatCols + ` FROM chats ORDER BY pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Chat{}
	for rows.Next() {
		c, err := scanChat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) GetChat(id string) (Chat, error) {
	c, err := scanChat(d.QueryRow(`SELECT `+chatCols+` FROM chats WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (d *DB) CreateChat(c *Chat) error {
	c.ID = NewID()
	c.CreatedAt = now()
	c.UpdatedAt = c.CreatedAt
	if c.Title == "" {
		c.Title = "New chat"
	}
	_, err := d.Exec(`INSERT INTO chats(`+chatCols+`) VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, c.Title, c.SystemPrompt, c.Model, c.LeafID, c.Pinned, c.CreatedAt, c.UpdatedAt)
	return err
}

// UpdateChat applies a partial update. Allowed keys: title, system_prompt, model, leaf_id, pinned.
func (d *DB) UpdateChat(id string, fields map[string]any, touch bool) error {
	allowed := map[string]bool{"title": true, "system_prompt": true, "model": true, "leaf_id": true, "pinned": true}
	sets, args := []string{}, []any{}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		sets = append(sets, k+"=?")
		args = append(args, v)
	}
	if touch {
		sets = append(sets, "updated_at=?")
		args = append(args, now())
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	res, err := d.Exec(`UPDATE chats SET `+strings.Join(sets, ",")+` WHERE id=?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) DeleteChat(id string) error {
	_, err := d.Exec(`DELETE FROM chats WHERE id=?`, id)
	return err
}

// ---------- messages ----------

type Message struct {
	ID        string   `json:"id"`
	ChatID    string   `json:"chat_id"`
	ParentID  string   `json:"parent_id"`
	Role      string   `json:"role"`
	Content   string   `json:"content"`
	Images    string   `json:"-"`
	ImageList []string `json:"images"`
	Model     string   `json:"model"`
	Error     string   `json:"error,omitempty"`
	CreatedAt int64    `json:"created_at"`
}

const msgCols = `id,chat_id,parent_id,role,content,images,model,error,created_at`

func (d *DB) ListMessages(chatID string) ([]Message, error) {
	rows, err := d.Query(`SELECT `+msgCols+` FROM messages WHERE chat_id=? ORDER BY created_at, rowid`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.ParentID, &m.Role, &m.Content, &m.Images, &m.Model, &m.Error, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.ImageList = decodeImages(m.Images)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) InsertMessage(m *Message) error {
	m.ID = NewID()
	m.CreatedAt = now()
	m.Images = encodeImages(m.ImageList)
	if m.ImageList == nil {
		m.ImageList = []string{}
	}
	_, err := d.Exec(`INSERT INTO messages(`+msgCols+`) VALUES(?,?,?,?,?,?,?,?,?)`,
		m.ID, m.ChatID, m.ParentID, m.Role, m.Content, m.Images, m.Model, m.Error, m.CreatedAt)
	return err
}

func (d *DB) FinishMessage(id, content, errText string) error {
	_, err := d.Exec(`UPDATE messages SET content=?, error=? WHERE id=?`, content, errText, id)
	return err
}

// Path returns messages from root to leafID (inclusive), in order.
func Path(all []Message, leafID string) []Message {
	byID := make(map[string]Message, len(all))
	for _, m := range all {
		byID[m.ID] = m
	}
	var rev []Message
	for id := leafID; id != ""; {
		m, ok := byID[id]
		if !ok {
			break
		}
		rev = append(rev, m)
		id = m.ParentID
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// ---------- search ----------

type SearchHit struct {
	ChatID  string `json:"chat_id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

func (d *DB) Search(q string) ([]SearchHit, error) {
	// quote each term so user input can't break FTS syntax; prefix-match the last one
	terms := strings.Fields(q)
	if len(terms) == 0 {
		return []SearchHit{}, nil
	}
	for i, t := range terms {
		terms[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
	}
	terms[len(terms)-1] += "*"
	match := strings.Join(terms, " ")

	rows, err := d.Query(`
		SELECT c.id, c.title, snippet(messages_fts, 0, '<mark>', '</mark>', '…', 12)
		FROM messages_fts f JOIN chats c ON c.id = f.chat_id
		WHERE messages_fts MATCH ?
		ORDER BY rank LIMIT 40`, match)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	out := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.ChatID, &h.Title, &h.Snippet); err != nil {
			return nil, err
		}
		if seen[h.ChatID] {
			continue
		}
		seen[h.ChatID] = true
		out = append(out, h)
	}
	// also match titles
	trows, err := d.Query(`SELECT id,title FROM chats WHERE title LIKE ? ORDER BY updated_at DESC LIMIT 20`, "%"+q+"%")
	if err == nil {
		defer trows.Close()
		for trows.Next() {
			var h SearchHit
			if trows.Scan(&h.ChatID, &h.Title) == nil && !seen[h.ChatID] {
				seen[h.ChatID] = true
				out = append([]SearchHit{h}, out...)
			}
		}
	}
	return out, rows.Err()
}

// ---------- presets ----------

type Preset struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prompt    string `json:"prompt"`
	CreatedAt int64  `json:"created_at"`
}

func (d *DB) ListPresets() ([]Preset, error) {
	rows, err := d.Query(`SELECT id,name,prompt,created_at FROM presets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Preset{}
	for rows.Next() {
		var p Preset
		if err := rows.Scan(&p.ID, &p.Name, &p.Prompt, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) SavePreset(p *Preset) error {
	if p.ID == "" {
		p.ID = NewID()
		p.CreatedAt = now()
		_, err := d.Exec(`INSERT INTO presets(id,name,prompt,created_at) VALUES(?,?,?,?)`, p.ID, p.Name, p.Prompt, p.CreatedAt)
		return err
	}
	_, err := d.Exec(`UPDATE presets SET name=?,prompt=? WHERE id=?`, p.Name, p.Prompt, p.ID)
	return err
}

func (d *DB) DeletePreset(id string) error {
	_, err := d.Exec(`DELETE FROM presets WHERE id=?`, id)
	return err
}
