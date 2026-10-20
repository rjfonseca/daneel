# Search Engine & Storage Architecture

Daneel combines a pure Go SQLite backend with the Full-Text Search 5 (FTS5) extension to deliver ranked lexical full-text queries across large knowledge vaults without external database dependencies.

---

## 1. Storage Engine & Database Layout

- **Zero CGO Compilation**: Uses `modernc.org/sqlite` for static Go compilation without external C toolchain requirements.
- **File Location**: Stored at `<vault_root>/.daneel/vault.db` or `$XDG_DATA_HOME/daneel/vault.db`.
- **Single-Writer Safety**: Coordinated via the Daneel daemon socket to prevent database lock contention during concurrent harvesting or indexing.

---

## 2. Relational & FTS5 Schema

```sql
-- Relational Note Metadata
CREATE TABLE IF NOT EXISTS notes (
    rel_path    TEXT PRIMARY KEY,
    title       TEXT,
    content     TEXT,
    category    TEXT,
    scope       TEXT,
    mod_time    INTEGER,
    hash        TEXT
);

-- Synaptic Cross-Link Graph
CREATE TABLE IF NOT EXISTS links (
    source_rel_path  TEXT,
    target_rel_path  TEXT,
    raw_link         TEXT,
    line_number      INTEGER
);

-- FTS5 Full-Text Virtual Table
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5 (
    title,
    content,
    rel_path,
    content='notes',
    content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 2'
);
```

---

## 3. BM25 Ranking and Relevance

Searches use SQLite's native BM25 scoring algorithm with weighted term matches:

- **Title Term Matches**: Weighted 5.0x higher than general note body content.
- **Diacritic Normalization**: Unicode 61 tokenizer automatically normalizes accented characters and accents (e.g. `ação` matches `acao`).
- **Snippet Highlighting**: Contextual snippets are extracted dynamically using SQLite's `snippet()` function, returning matching sentences wrapped with `<mark>` tags.

---

## 4. Incremental Indexing Pipeline

To eliminate redundant disk I/O, the indexer checks content hashes before re-indexing:
1. Calculates hash of file content.
2. Compares with stored hash in the `notes` table.
3. If unchanged and file modification time matches, skips re-indexing.
4. If changed, updates relational metadata, regenerates link graph entries, and syncs the FTS5 virtual table.
