package transfer

const (
	StatusRunning     = "running"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
	StatusInterrupted = "interrupted"
)

// QueueRecord is a persisted unfinished transfer.
type QueueRecord struct {
	ID           string
	OwnerKey     string
	TransferType string
	LocalFile    string
	RemoteFile   string
	TotalSize    int64
	Status       string
	Error        string
	// Overwrite is set for in-place edit transfers. Retry must replace the
	// destination instead of resuming by size.
	Overwrite bool
}

// QueueStore persists unfinished transfers across restarts.
type QueueStore interface {
	Upsert(rec QueueRecord) error
	Delete(id string) error
	Get(id string) (*QueueRecord, error)
	ListByOwner(ownerKey string) ([]QueueRecord, error)
	SetStatus(id, status, errMsg string) error
}

// RecordToProgress maps a queue row to UI progress (terminal / retryable).
func RecordToProgress(rec QueueRecord, sessionID string) ProgressData {
	errMsg := rec.Error
	if errMsg == "" {
		errMsg = rec.Status
	}
	return ProgressData{
		ID:           rec.ID,
		SessionID:    sessionID,
		OwnerKey:     rec.OwnerKey,
		TransferType: rec.TransferType,
		LocalFile:    rec.LocalFile,
		RemoteFile:   rec.RemoteFile,
		TotalSize:    rec.TotalSize,
		Rate:         0,
		Done:         true,
		Error:        errMsg,
	}
}
