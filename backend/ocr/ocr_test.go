package ocr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandReadsMarkedLines(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fakeocr")
	script := "#!/bin/sh\n" +
		"echo noise from the model\n" +
		"echo 'SERVYS_OCR {\"lines\":[\"Пробег: 45 230 км\",\"Замена масла двигателя\"]}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := (Command{Python: bin}).Read(context.Background(), filepath.Join(dir, "order.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "45 230 км") || !strings.Contains(got, "Замена масла") {
		t.Fatalf("%q", got)
	}
}

func TestCommandStopsWhenContextEnds(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "sleepocr")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := (Command{Python: bin}).Read(ctx, "order.jpg")
	if err == nil {
		t.Fatal("ожидали остановку по отмене")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("процесс чтения не остановился")
	}
}
