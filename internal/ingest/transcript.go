package ingest

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

type transcriptEntry struct {
	Type    string `json:"type"`
	UUID    string `json:"uuid"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// TranscriptDelta reads a Claude Code JSONL transcript and returns a readable digest
// of every message after `cursor` (a 0-based count of already-processed lines), the
// new cursor, the last message UUID seen, and whether the whole file contains any
// human (user) turn. The delta keeps each per-turn run small and avoids reprocessing
// the entire conversation every time.
func TranscriptDelta(path string, cursor int) (digest string, newCursor int, lastUUID string, hasHuman bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", cursor, "", false, err
	}
	defer f.Close()

	var b strings.Builder
	idx := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 32*1024*1024) // transcripts can have very long lines
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e transcriptEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			idx++
			continue
		}
		if e.Type == "user" && e.Message.Role == "user" {
			hasHuman = true
		}
		if idx >= cursor {
			if text := extractText(e); text != "" {
				role := e.Message.Role
				if role == "" {
					role = e.Type
				}
				b.WriteString(strings.ToUpper(role))
				b.WriteString(":\n")
				b.WriteString(text)
				b.WriteString("\n\n")
			}
			if e.UUID != "" {
				lastUUID = e.UUID
			}
		}
		idx++
	}
	if err := sc.Err(); err != nil {
		return "", cursor, "", hasHuman, err
	}
	return b.String(), idx, lastUUID, hasHuman, nil
}

// extractText pulls human-readable text out of a message's content, which may be a
// bare string or an array of content blocks. Tool results are skipped as noise; tool
// uses are reduced to a short marker so the digest stays focused on the conversation.
func extractText(e transcriptEntry) string {
	if len(e.Message.Content) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(e.Message.Content, &s) == nil {
		return strings.TrimSpace(s)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
		Name string `json:"name"`
	}
	if json.Unmarshal(e.Message.Content, &blocks) == nil {
		var parts []string
		for _, bl := range blocks {
			switch bl.Type {
			case "text":
				if t := strings.TrimSpace(bl.Text); t != "" {
					parts = append(parts, t)
				}
			case "tool_use":
				if bl.Name != "" {
					parts = append(parts, "[tool: "+bl.Name+"]")
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}
