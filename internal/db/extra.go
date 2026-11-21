package db

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"sort"
)

const extraSchema = `
CREATE TABLE IF NOT EXISTS mcp_servers (
	id         TEXT PRIMARY KEY,
	config     TEXT NOT NULL,   -- JSON (mcp.Config without id)
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS embeddings (
	message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
	chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
	model      TEXT NOT NULL,
	vec        BLOB NOT NULL     -- little-endian float32, L2-normalized
);
CREATE INDEX IF NOT EXISTS embeddings_model ON embeddings(model);
`

func (d *DB) migrateExtra() error {
	_, err := d.Exec(extraSchema)
	return err
}

// ---------- MCP servers ----------

type MCPServer struct {
	ID        string
	Config    json.RawMessage
	CreatedAt int64
}

func (d *DB) ListMCPServers() ([]MCPServer, error) {
	rows, err := d.Query(`SELECT id, config, created_at FROM mcp_servers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MCPServer
	for rows.Next() {
		var s MCPServer
		var cfg string
		if err := rows.Scan(&s.ID, &cfg, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.Config = json.RawMessage(cfg)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) GetMCPServer(id string) (MCPServer, error) {
	var s MCPServer
	var cfg string
	err := d.QueryRow(`SELECT id, config, created_at FROM mcp_servers WHERE id=?`, id).Scan(&s.ID, &cfg, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	s.Config = json.RawMessage(cfg)
	return s, err
}

func (d *DB) SaveMCPServer(s *MCPServer) error {
	if s.ID == "" {
		s.ID = NewID()
		s.CreatedAt = now()
		_, err := d.Exec(`INSERT INTO mcp_servers(id, config, created_at) VALUES(?,?,?)`, s.ID, string(s.Config), s.CreatedAt)
		return err
	}
	_, err := d.Exec(`UPDATE mcp_servers SET config=? WHERE id=?`, string(s.Config), s.ID)
	return err
}

func (d *DB) DeleteMCPServer(id string) error {
	_, err := d.Exec(`DELETE FROM mcp_servers WHERE id=?`, id)
	return err
}

// ---------- embeddings ----------

// PendingEmbed is an assistant reply (with its question) that has no vector yet.
type PendingEmbed struct {
	MessageID, ChatID, Title, Question, Answer string
}

func (d *DB) PendingEmbeds(model string, exclude map[string]bool, limit int) ([]PendingEmbed, error) {
	rows, err := d.Query(`
		SELECT m.id, m.chat_id, c.title, COALESCE(u.content, ''), m.content
		FROM messages m
		JOIN chats c ON c.id = m.chat_id
		LEFT JOIN messages u ON u.id = m.parent_id
		LEFT JOIN embeddings e ON e.message_id = m.id AND e.model = ?
		WHERE m.role = 'assistant' AND m.content != '' AND m.error = '' AND e.message_id IS NULL
		ORDER BY m.created_at DESC
		LIMIT ?`, model, limit+len(exclude))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingEmbed
	for rows.Next() {
		var p PendingEmbed
		if err := rows.Scan(&p.MessageID, &p.ChatID, &p.Title, &p.Question, &p.Answer); err != nil {
			return nil, err
		}
		if exclude[p.MessageID] {
			continue
		}
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

func (d *DB) SaveEmbedding(messageID, chatID, model string, vec []float32) error {
	_, err := d.Exec(`INSERT INTO embeddings(message_id, chat_id, model, vec) VALUES(?,?,?,?)
		ON CONFLICT(message_id) DO UPDATE SET model=excluded.model, vec=excluded.vec`,
		messageID, chatID, model, encodeVec(normalize(vec)))
	return err
}

func (d *DB) DropEmbeddingsExcept(model string) error {
	_, err := d.Exec(`DELETE FROM embeddings WHERE model != ?`, model)
	return err
}

func (d *DB) EmbedCounts(model string) (indexed, total int, err error) {
	if err = d.QueryRow(`SELECT COUNT(*) FROM embeddings WHERE model=?`, model).Scan(&indexed); err != nil {
		return
	}
	err = d.QueryRow(`SELECT COUNT(*) FROM messages WHERE role='assistant' AND content != '' AND error = ''`).Scan(&total)
	return
}

type SemanticHit struct {
	ChatID    string  `json:"chat_id"`
	Title     string  `json:"title"`
	Snippet   string  `json:"snippet"`
	MessageID string  `json:"message_id"`
	Score     float32 `json:"score"`
}

// SemanticSearch scores every stored vector against q (brute force; fine
// for tens of thousands of messages) and returns the best match per chat.
func (d *DB) SemanticSearch(model string, q []float32, limit int) ([]SemanticHit, error) {
	q = normalize(q)
	rows, err := d.Query(`SELECT message_id, chat_id, vec FROM embeddings WHERE model=?`, model)
	if err != nil {
		return nil, err
	}
	best := map[string]SemanticHit{}
	var blob []byte
	for rows.Next() {
		var h SemanticHit
		if err := rows.Scan(&h.MessageID, &h.ChatID, &blob); err != nil {
			rows.Close()
			return nil, err
		}
		if len(blob) != 4*len(q) {
			continue // dimension mismatch (model changed under us)
		}
		h.Score = dotBlob(q, blob)
		if prev, ok := best[h.ChatID]; !ok || h.Score > prev.Score {
			best[h.ChatID] = h
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	hits := make([]SemanticHit, 0, len(best))
	for _, h := range best {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	for i := range hits {
		var q, a string
		_ = d.QueryRow(`SELECT c.title, COALESCE(u.content,''), m.content FROM messages m
			JOIN chats c ON c.id=m.chat_id LEFT JOIN messages u ON u.id=m.parent_id WHERE m.id=?`,
			hits[i].MessageID).Scan(&hits[i].Title, &q, &a)
		if q == "" {
			q = a
		}
		hits[i].Snippet = q
	}
	return hits, nil
}

func normalize(v []float32) []float32 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if s == 0 {
		return v
	}
	n := float32(1 / math.Sqrt(s))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * n
	}
	return out
}

func encodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
	}
	return b
}

func dotBlob(q []float32, b []byte) float32 {
	var s float32
	for i := range q {
		s += q[i] * math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return s
}

// ---------- import / export ----------

type ChatExport struct {
	Chat     Chat      `json:"chat"`
	Messages []Message `json:"messages"`
}

func (d *DB) ExportAll() ([]ChatExport, error) {
	chats, err := d.ListChats()
	if err != nil {
		return nil, err
	}
	out := make([]ChatExport, 0, len(chats))
	for _, c := range chats {
		ms, err := d.ListMessages(c.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, ChatExport{Chat: c, Messages: ms})
	}
	return out, nil
}

// ImportChat inserts a chat and its messages as given (ids already fresh,
// parents already mapped, timestamps preserved).
func (d *DB) ImportChat(c Chat, msgs []Message) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO chats(`+chatCols+`) VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, c.Title, c.SystemPrompt, c.Model, c.LeafID, c.Pinned, c.CreatedAt, c.UpdatedAt); err != nil {
		return err
	}
	for _, m := range msgs {
		if _, err := tx.Exec(`INSERT INTO messages(`+msgCols+`) VALUES(?,?,?,?,?,?,?,?,?)`,
			m.ID, c.ID, m.ParentID, m.Role, m.Content, encodeImages(m.ImageList), m.Model, m.Error, m.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) SaveMessageContent(id, content string) error {
	_, err := d.Exec(`UPDATE messages SET content=? WHERE id=?`, content, id)
	return err
}
