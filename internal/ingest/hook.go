package ingest

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"syscall"
)

// RunHook is invoked by the Claude Code Stop hook. It reads the hook payload from
// stdin, applies cheap guards, enqueues a job, and spawns the dispatcher detached —
// then returns immediately so the user's session never blocks on ingestion.
func RunHook() error {
	// Recursion guard: the steered ingestion session runs with MULTI_INGEST=1,
	// so its own Stop event must be a no-op.
	if os.Getenv("MULTI_INGEST") != "" {
		return nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil // never break the host session
	}
	var hd struct {
		SessionID      string `json:"session_id"`
		TranscriptPath string `json:"transcript_path"`
		Cwd            string `json:"cwd"`
	}
	if json.Unmarshal(data, &hd) != nil {
		return nil
	}
	if hd.SessionID == "" || hd.TranscriptPath == "" {
		return nil
	}
	if _, err := os.Stat(hd.TranscriptPath); err != nil {
		return nil
	}
	if _, err := Enqueue(Job{SessionID: hd.SessionID, TranscriptPath: hd.TranscriptPath, Cwd: hd.Cwd}); err != nil {
		return nil
	}
	spawnDispatcher()
	return nil
}

// spawnDispatcher starts `ingester dispatch` in a detached session (fire-and-forget).
func spawnDispatcher() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "dispatch")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = cmd.Start()
}
