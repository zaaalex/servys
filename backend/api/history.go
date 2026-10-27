package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zaaalex/servys/backend/lane"
	"github.com/zaaalex/servys/backend/maintenance"
)

func (s *Server) postHistory(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", "нужен текст истории")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "вставьте текст сервисной книжки")
		return
	}
	note, err := s.Store.AddNote(r.Context(), v.AccountID, v.ID, text, "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", "не удалось сохранить текст")
		return
	}
	writeJSON(w, http.StatusCreated, historyJSON(note.ID, text, ""))
}

func (s *Server) listHistory(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	notes, err := s.Store.ListNotes(r.Context(), v.AccountID, v.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", "не удалось прочитать историю")
		return
	}
	out := make([]map[string]any, 0, len(notes))
	for _, n := range notes {
		out = append(out, map[string]any{
			"id": n.ID, "body": n.Body, "filename": n.Filename, "created_at": n.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": out})
}

func (s *Server) postHistoryFile(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "файл больше 8 МБ или форма повреждена")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "приложите файл в поле file")
		return
	}
	defer file.Close()
	name := filepath.Base(hdr.Filename)
	ext := strings.ToLower(filepath.Ext(name))
	kind := fileKind(ext, hdr.Header.Get("Content-Type"))
	if kind == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "принимаются txt, pdf, jpg, png и webp")
		return
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "файл не прочитан")
		return
	}
	dir := s.UploadDir
	if dir == "" {
		dir = "data/uploads"
	}
	idDir := filepath.Join(dir, v.ID)
	if err := os.MkdirAll(idDir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", "не удалось сохранить файл")
		return
	}
	stored := filepath.Join(idDir, time.Now().UTC().Format("20060102T150405")+"-"+sanitize(name))
	if err := os.WriteFile(stored, raw, 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", "не удалось сохранить файл")
		return
	}
	body := "Файл сохранён: " + name + ". Текст с фото и pdf здесь не читается — вставьте строки книжки текстом."
	parseText := ""
	notice := ""
	if kind == "text" && utf8.Valid(raw) {
		parseText = string(raw)
		body = parseText
	} else if kind == "file" {
		body, parseText, notice = s.readScan(r.Context(), stored, name)
	}
	note, err := s.Store.AddNote(r.Context(), v.AccountID, v.ID, body, name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", "не удалось записать файл в историю")
		return
	}
	writeJSON(w, http.StatusCreated, historyJSON(note.ID, parseText, notice))
}

func (s *Server) readScan(ctx context.Context, stored, name string) (body, parseText, notice string) {
	if s.Scan == nil {
		notice = "Снимок сохранён. Чтение не настроено — отметьте работы сами."
		return notice, "", notice
	}
	var text string
	var scanErr error
	run := func(ctx context.Context) error {
		text, scanErr = s.Scan.Read(ctx, stored)
		return nil
	}
	var err error
	if s.Lane != nil {
		err = s.Lane.Do(ctx, lane.KindOCR, run)
	} else {
		err = run(ctx)
	}
	if err != nil || scanErr != nil || strings.TrimSpace(text) == "" {
		if scanErr != nil {
			log.Printf("ocr: %s: %v", name, scanErr)
		} else if err != nil {
			log.Printf("ocr: %s: %v", name, err)
		}
		notice = "Снимок сохранён. Текст не прочитался — отметьте работы сами."
		return notice, "", notice
	}
	return text, text, ""
}

func historyJSON(noteID, text, notice string) map[string]any {
	items := maintenance.ParseDocument(text, time.Now().UTC())
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{
			"rule_code": item.RuleCode,
			"title":     item.Title,
			"odometer":  item.Odometer,
			"source":    item.Source,
		}
		if !item.PerformedAt.IsZero() {
			row["performed_at"] = item.PerformedAt.Format("2006-01-02")
		}
		out = append(out, row)
	}
	resp := map[string]any{"note_id": noteID, "suggestions": out}
	if notice != "" {
		resp["notice"] = notice
	}
	return resp
}

func fileKind(ext, mime string) string {
	switch ext {
	case ".txt":
		return "text"
	case ".pdf":
		return "file"
	case ".jpg", ".jpeg", ".png", ".webp":
		return "file"
	}
	mime = strings.ToLower(mime)
	switch {
	case strings.HasPrefix(mime, "text/plain"):
		return "text"
	case mime == "application/pdf", mime == "image/jpeg", mime == "image/png", mime == "image/webp":
		return "file"
	default:
		return ""
	}
}

func sanitize(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < 32 {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}

func parseWhen(raw string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, raw)
}
