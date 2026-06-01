package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Job is a queued ingestion request: written by the hook, consumed by the dispatcher.
type Job struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	EnqueuedAt     string `json:"enqueued_at"`

	file string
}

// Enqueue writes a job file. Repeated Stop firings for one session just add more job
// files; the dispatcher coalesces them (newest transcript wins).
func Enqueue(j Job) (string, error) {
	if err := ensureDirs(); err != nil {
		return "", err
	}
	j.EnqueuedAt = nowStamp()
	p := filepath.Join(queueDir(), fileStamp()+"_"+sanitize(j.SessionID)+".json")
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// PendingJobs returns queued jobs sorted oldest-first (by filename timestamp).
func PendingJobs() ([]Job, error) {
	entries, err := os.ReadDir(queueDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var jobs []Job
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p := filepath.Join(queueDir(), e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var j Job
		if json.Unmarshal(data, &j) != nil {
			continue
		}
		j.file = p
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].file < jobs[k].file })
	return jobs, nil
}

// complete moves a job out of the queue into done/ (success) or failed/ (dead-letter).
func (j Job) complete(success bool) {
	if j.file == "" {
		return
	}
	dst := doneDir()
	if !success {
		dst = failedDir()
	}
	_ = os.Rename(j.file, filepath.Join(dst, filepath.Base(j.file)))
}
