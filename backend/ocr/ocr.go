// Package ocr запускает PaddleOCR отдельным процессом и дожидается его выхода.
// Между снимками процесс не остаётся в памяти.
package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const marker = "SERVYS_OCR "

type Command struct {
	Python string
	Script string
}

func (c Command) Read(ctx context.Context, path string) (string, error) {
	if strings.TrimSpace(c.Python) == "" {
		return "", errors.New("ocr: не задан интерпретатор")
	}
	args := make([]string, 0, 2)
	if c.Script != "" {
		args = append(args, c.Script)
	}
	args = append(args, path)
	cmd := exec.CommandContext(ctx, c.Python, args...)
	cmd.Env = append(os.Environ(),
		"GLOG_minloglevel=3",
		"FLAGS_minloglevel=3",
		"PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	payload, ok := marked(out)
	if !ok {
		if err != nil {
			return "", fmt.Errorf("ocr: %w: %s", err, tail(out))
		}
		return "", errors.New("ocr: пустой ответ")
	}
	if payload.Error != "" {
		return "", errors.New(payload.Error)
	}
	if err != nil && len(payload.Lines) == 0 {
		return "", fmt.Errorf("ocr: %w: %s", err, tail(out))
	}
	return strings.Join(payload.Lines, "\n"), nil
}

type markedPayload struct {
	Lines []string `json:"lines"`
	Error string   `json:"error"`
}

func marked(out []byte) (markedPayload, bool) {
	var payload markedPayload
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		raw := bytes.TrimSpace(line)
		if !bytes.HasPrefix(raw, []byte(marker)) {
			continue
		}
		if err := json.Unmarshal(raw[len(marker):], &payload); err != nil {
			return markedPayload{}, false
		}
		return payload, true
	}
	return markedPayload{}, false
}

func tail(out []byte) string {
	text := strings.TrimSpace(string(out))
	if len(text) > 400 {
		text = text[len(text)-400:]
	}
	return text
}

// FindScript ищет read.py от текущей папки или от каталога бинарника.
func FindScript() string {
	if p := os.Getenv("OCR_SCRIPT"); p != "" {
		return p
	}
	candidates := []string{
		filepath.Join("ocr", "read.py"),
		filepath.Join("backend", "ocr", "read.py"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "ocr", "read.py"))
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("ocr", "read.py")
}

// Python выбирает интерпретатор рядом со скриптом, если окружение уже поставлено.
func Python(script string) string {
	if p := os.Getenv("OCR_PYTHON"); p != "" {
		return p
	}
	venv := filepath.Join(filepath.Dir(script), ".venv", "bin", "python")
	info, err := os.Stat(venv)
	if err == nil && !info.IsDir() {
		return venv
	}
	return "python3"
}
