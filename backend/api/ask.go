package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zaaalex/servys/backend/recommender"
)

type askReq struct {
	Question string `json:"question"`
}

func (s *Server) askVehicle(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	var req askReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	question := strings.TrimSpace(req.Question)
	n := utf8.RuneCountInString(question)
	if n < 1 || n > 300 {
		writeErr(w, http.StatusBadRequest, "BAD_QUESTION", "вопрос от 1 до 300 знаков")
		return
	}
	if s.Ask == nil {
		writeErr(w, http.StatusBadGateway, "ADVISOR_ERROR", recommender.ErrAskOff.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Minute)
	defer cancel()
	reply, err := s.Ask.Ask(ctx, v, question)
	if err != nil {
		log.Printf("ask: %v", err)
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "поиск по вопросу не успел ответить"
		}
		writeErr(w, http.StatusBadGateway, "ADVISOR_ERROR", msg)
		return
	}
	sources := make([]map[string]string, 0, len(reply.Sources))
	for _, c := range reply.Sources {
		sources = append(sources, map[string]string{"title": c.Title, "url": c.URL})
	}
	writeJSON(w, http.StatusOK, map[string]any{"answer": reply.Text, "sources": sources})
}
