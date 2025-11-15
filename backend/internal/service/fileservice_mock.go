package service

import (
	"github.com/Muneer320/RhinoBox/internal/storage"
)

// MockFileService is a mock implementation of FileService for testing
type MockFileService struct {
	StoreFileFunc              func(req storage.StoreRequest) (*storage.StoreResult, error)
	RenameFileFunc             func(req storage.RenameRequest) (*storage.RenameResult, error)
	DeleteFileFunc             func(req storage.DeleteRequest) (*storage.DeleteResult, error)
	UpdateFileMetadataFunc      func(req storage.MetadataUpdateRequest) (*storage.MetadataUpdateResult, error)
	BatchUpdateFileMetadataFunc func(updates []storage.MetadataUpdateRequest) ([]storage.MetadataUpdateResult, []error)
	GetFileByHashFunc          func(hash string) (*storage.FileRetrievalResult, error)
	GetFileByPathFunc          func(path string) (*storage.FileRetrievalResult, error)
	GetFileMetadataFunc         func(hash string) (*storage.FileMetadata, error)
	SearchFilesFunc            func(query string) []storage.FileMetadata
	LogDownloadFunc            func(log storage.DownloadLog) error
}

func (m *MockFileService) StoreFile(req storage.StoreRequest) (*storage.StoreResult, error) {
	if m.StoreFileFunc != nil {
		return m.StoreFileFunc(req)
	}
	return nil, nil
}

func (m *MockFileService) RenameFile(req storage.RenameRequest) (*storage.RenameResult, error) {
	if m.RenameFileFunc != nil {
		return m.RenameFileFunc(req)
	}
	return nil, nil
}

func (m *MockFileService) DeleteFile(req storage.DeleteRequest) (*storage.DeleteResult, error) {
	if m.DeleteFileFunc != nil {
		return m.DeleteFileFunc(req)
	}
	return nil, nil
}

func (m *MockFileService) UpdateFileMetadata(req storage.MetadataUpdateRequest) (*storage.MetadataUpdateResult, error) {
	if m.UpdateFileMetadataFunc != nil {
		return m.UpdateFileMetadataFunc(req)
	}
	return nil, nil
}

func (m *MockFileService) BatchUpdateFileMetadata(updates []storage.MetadataUpdateRequest) ([]storage.MetadataUpdateResult, []error) {
	if m.BatchUpdateFileMetadataFunc != nil {
		return m.BatchUpdateFileMetadataFunc(updates)
	}
	return nil, nil
}

func (m *MockFileService) GetFileByHash(hash string) (*storage.FileRetrievalResult, error) {
	if m.GetFileByHashFunc != nil {
		return m.GetFileByHashFunc(hash)
	}
	return nil, nil
}

func (m *MockFileService) GetFileByPath(path string) (*storage.FileRetrievalResult, error) {
	if m.GetFileByPathFunc != nil {
		return m.GetFileByPathFunc(path)
	}
	return nil, nil
}

func (m *MockFileService) GetFileMetadata(hash string) (*storage.FileMetadata, error) {
	if m.GetFileMetadataFunc != nil {
		return m.GetFileMetadataFunc(hash)
	}
	return nil, nil
}

func (m *MockFileService) SearchFiles(query string) []storage.FileMetadata {
	if m.SearchFilesFunc != nil {
		return m.SearchFilesFunc(query)
	}
	return nil
}

func (m *MockFileService) LogDownload(log storage.DownloadLog) error {
	if m.LogDownloadFunc != nil {
		return m.LogDownloadFunc(log)
	}
	return nil
}


