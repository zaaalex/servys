package api

import (
	"io"
	"net/http"

	"github.com/zaaalex/servys/backend/messenger"
)

func (s *Server) telegramWebhook(w http.ResponseWriter, r *http.Request) {
	s.deliverWebhook(w, r, messenger.HandleTelegram, "X-Telegram-Bot-Api-Secret-Token")
}

func (s *Server) maxWebhook(w http.ResponseWriter, r *http.Request) {
	s.deliverWebhook(w, r, messenger.HandleMax, "X-Max-Bot-Api-Secret")
}

func (s *Server) deliverWebhook(w http.ResponseWriter, r *http.Request, handle func(messenger.Config, string, []byte) messenger.Outcome, secretHeader string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_BODY", "тело запроса не прочитано")
		return
	}
	out := handle(s.Messenger, r.Header.Get(secretHeader), body)
	if out.Send != nil && s.Send != nil {
		if err := s.Send.Send(r.Context(), *out.Send); err != nil {
			writeErr(w, http.StatusBadGateway, "SEND_FAILED", "не удалось отправить ответ бота")
			return
		}
	}
	w.WriteHeader(out.Status)
}
