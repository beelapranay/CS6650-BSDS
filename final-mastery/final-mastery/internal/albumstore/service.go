package albumstore

import (
	"io"
	"log"
	"sync"

	"github.com/google/uuid"
)

type Service struct {
	albums         AlbumRepository
	photos         PhotoRepository
	files          FileStorage
	dispatcher     BackgroundDispatcher
	maxUploadBytes int64
}

func NewService(albums AlbumRepository, photos PhotoRepository, files FileStorage, dispatcher BackgroundDispatcher, maxUploadBytes int64) *Service {
	return &Service{
		albums:         albums,
		photos:         photos,
		files:          files,
		dispatcher:     dispatcher,
		maxUploadBytes: maxUploadBytes,
	}
}

func (s *Service) PutAlbum(payload map[string]any) (int, map[string]any, error) {
	required := []string{"album_id", "title", "description", "owner"}
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			return 0, nil, NewAppError(400, map[string]any{"error": "missing required field"})
		}
		if _, ok := payload[field].(string); !ok {
			return 0, nil, NewAppError(400, map[string]any{"error": "invalid " + field})
		}
	}

	album := Album{
		AlbumID:     payload["album_id"].(string),
		Title:       payload["title"].(string),
		Description: payload["description"].(string),
		Owner:       payload["owner"].(string),
	}

	created, err := s.albums.Upsert(album)
	if err != nil {
		return 0, nil, err
	}
	if created {
		return 201, serializeAlbum(album), nil
	}
	return 200, serializeAlbum(album), nil
}

func (s *Service) GetAlbum(albumID string) (map[string]any, error) {
	album, err := s.albums.Get(albumID)
	if err != nil {
		return nil, err
	}
	if album == nil {
		return nil, NewAppError(404, map[string]any{"error": "not found"})
	}
	return serializeAlbum(*album), nil
}

func (s *Service) ListAlbums() ([]map[string]any, error) {
	albums, err := s.albums.ListAll()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(albums))
	for _, album := range albums {
		out = append(out, serializeAlbum(album))
	}
	return out, nil
}

func (s *Service) UploadPhoto(albumID string, file io.Reader, originalFilename string) (map[string]any, error) {
	exists, err := s.albums.Exists(albumID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, NewAppError(404, map[string]any{"error": "not found"})
	}

	photoID := uuid.NewString()

	// Run AllocateSeq in parallel with S3 upload
	var seq int64
	var seqErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		seq, seqErr = s.photos.AllocateSeq(albumID)
	}()

	stagedUpload, uploadErr := s.files.StageUpload(file, photoID, originalFilename, s.maxUploadBytes)

	wg.Wait()

	if seqErr != nil {
		if uploadErr == nil {
			_ = s.files.DeleteFile(stagedUpload.TempPath)
		}
		return nil, seqErr
	}
	if uploadErr != nil {
		return nil, mapUploadError(uploadErr)
	}

	if err := s.photos.CreateProcessingPhoto(photoID, albumID, seq, stagedUpload); err != nil {
		_ = s.files.DeleteFile(stagedUpload.TempPath)
		return nil, err
	}

	if err := s.dispatcher.EnqueuePhotoProcessing(albumID, photoID); err != nil {
		_ = s.photos.MarkFailed(albumID, photoID)
		return nil, err
	}

	return map[string]any{
		"photo_id": photoID,
		"seq":      seq,
		"status":   "processing",
	}, nil
}

func (s *Service) ProcessPhoto(albumID, photoID string) {
	photo, err := s.photos.Get(albumID, photoID)
	if err != nil || photo == nil {
		return
	}

	if photo.TempPath == "" {
		_ = s.photos.MarkFailed(albumID, photoID)
		return
	}

	storedPath, err := s.files.Promote(StagedUpload{
		OriginalFilename: photo.OriginalFilename,
		TempPath:         photo.TempPath,
	}, photoID)
	if err != nil {
		log.Printf("process photo failed album_id=%s photo_id=%s err=%v", albumID, photoID, err)
		_ = s.photos.MarkFailed(albumID, photoID)
		return
	}

	updated, err := s.photos.MarkCompleted(albumID, photoID, storedPath)
	if err != nil {
		log.Printf("mark completed failed album_id=%s photo_id=%s err=%v", albumID, photoID, err)
		_ = s.photos.MarkFailed(albumID, photoID)
		return
	}
	if !updated {
		_ = s.files.DeleteFile(storedPath)
	}
}

func (s *Service) GetPhoto(albumID, photoID, baseURL string) (map[string]any, error) {
	photo, err := s.photos.Get(albumID, photoID)
	if err != nil {
		return nil, err
	}
	if photo == nil {
		return nil, NewAppError(404, map[string]any{"error": "not found"})
	}
	payload := map[string]any{
		"photo_id": photo.PhotoID,
		"album_id": photo.AlbumID,
		"seq":      photo.Seq,
		"status":   photo.Status,
	}
	if photo.Status == "completed" {
		payload["url"] = s.files.BuildPublicURL(photo.StoragePath, baseURL, photo.PhotoID)
	}
	return payload, nil
}

func (s *Service) DeletePhoto(albumID, photoID string) error {
	record, err := s.photos.Delete(albumID, photoID)
	if err != nil {
		return err
	}
	if record == nil {
		return NewAppError(404, map[string]any{"error": "not found"})
	}
	_ = s.files.DeleteFile(record.TempPath)
	_ = s.files.DeleteFile(record.StoragePath)
	return nil
}

func (s *Service) ResolveStoredPhoto(photoID string) (*PhotoRecord, *StoredFile, io.ReadCloser, error) {
	photo, err := s.photos.GetByPhotoID(photoID)
	if err != nil {
		return nil, nil, nil, err
	}
	if photo == nil || photo.Status != "completed" || photo.StoragePath == "" {
		return nil, nil, nil, NewAppError(404, map[string]any{"error": "not found"})
	}
	reader, meta, err := s.files.OpenStoredFile(photo.StoragePath)
	if err != nil || meta == nil {
		return nil, nil, nil, NewAppError(404, map[string]any{"error": "not found"})
	}
	return photo, meta, reader, nil
}

func (s *Service) Close() error {
	return s.dispatcher.Close()
}

func serializeAlbum(album Album) map[string]any {
	return map[string]any{
		"album_id":    album.AlbumID,
		"title":       album.Title,
		"description": album.Description,
		"owner":       album.Owner,
	}
}

func mapUploadError(err error) error {
	var appErr *AppError
	if ok := AsAppError(err, &appErr); ok {
		return appErr
	}
	if err != nil && err.Error() == "payload too large" {
		return NewAppError(413, map[string]any{"error": "payload too large"})
	}
	return err
}

func AsAppError(err error, target **AppError) bool {
	appErr, ok := err.(*AppError)
	if ok && target != nil {
		*target = appErr
	}
	return ok
}
