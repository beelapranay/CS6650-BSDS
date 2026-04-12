package albumstore

import (
	"errors"
	"io"
)

type AppError struct {
	StatusCode int
	Payload    map[string]any
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if message, ok := e.Payload["error"].(string); ok && message != "" {
		return message
	}
	return "application error"
}

func NewAppError(status int, payload map[string]any) *AppError {
	return &AppError{StatusCode: status, Payload: payload}
}

type Album struct {
	AlbumID     string `json:"album_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

type AcceptedPhoto struct {
	PhotoID string `json:"photo_id"`
	Seq     int64  `json:"seq"`
	Status  string `json:"status"`
}

type PhotoRecord struct {
	PhotoID          string
	AlbumID          string
	Seq              int64
	Status           string
	OriginalFilename string
	TempPath         string
	StoragePath      string
}

type StagedUpload struct {
	OriginalFilename string
	TempPath         string
}

type StoredFile struct {
	Path          string
	ContentType   string
	ContentLength int64
}

type AlbumRepository interface {
	Upsert(album Album) (bool, error)
	Get(albumID string) (*Album, error)
	ListAll() ([]Album, error)
	Exists(albumID string) (bool, error)
}

type PhotoRepository interface {
	AllocateSeq(albumID string) (int64, error)
	CreateProcessingPhoto(photoID, albumID string, seq int64, stagedUpload StagedUpload) error
	Get(albumID, photoID string) (*PhotoRecord, error)
	GetByPhotoID(photoID string) (*PhotoRecord, error)
	MarkCompleted(albumID, photoID, storagePath string) (bool, error)
	MarkFailed(albumID, photoID string) error
	Delete(albumID, photoID string) (*PhotoRecord, error)
}

type FileStorage interface {
	StageUpload(file io.Reader, photoID, originalFilename string, maxUploadBytes int64) (StagedUpload, error)
	Promote(stagedUpload StagedUpload, photoID string) (string, error)
	DeleteFile(path string) error
	StatStoredFile(path string) (*StoredFile, error)
	OpenStoredFile(path string) (io.ReadCloser, *StoredFile, error)
	BuildPublicURL(storagePath, baseURL, photoID string) string
}

type BackgroundDispatcher interface {
	EnqueuePhotoProcessing(albumID, photoID string) error
	Close() error
}

var ErrNotFound = errors.New("not found")
