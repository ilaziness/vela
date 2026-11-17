package services

// ListPasteConflicts returns destination basenames under destDir that already
// exist among srcPaths, so the frontend can offer an overwrite confirm.
func (s *SftpService) ListPasteConflicts(sessionID, destDir string, srcPaths []string) ([]string, error) {
	return s.mgr.PasteConflicts(sessionID, destDir, srcPaths)
}

// PasteRemotePaths copies remote entries into destDir; destNames optionally
// renames entries at the target (same length as srcPaths, or nil for basenames).
// With cut the sources are removed after their copy succeeds. Each entry is
// pasted by one silent remote cp/mv command when the server allows command
// execution (data stays on the server), falling back to a streaming copy over
// this SFTP connection otherwise. Progress shows up in the transfer panel.
func (s *SftpService) PasteRemotePaths(sessionID string, srcPaths []string, destNames []string, destDir string, cut bool, overwrite bool) error {
	return s.mgr.PasteRemotePaths(sessionID, srcPaths, destNames, destDir, cut, overwrite)
}

// CompressRemotePaths builds a tar.gz archive of srcPaths at archivePath on the
// server with a remote tar command — the data never leaves it and the
// interactive terminal is not involved.
func (s *SftpService) CompressRemotePaths(sessionID string, srcPaths []string, archivePath string) error {
	return s.mgr.CompressRemotePaths(sessionID, srcPaths, archivePath)
}

// ListArchiveConflicts returns flattened target paths under destDir that already
// exist, so the frontend can offer an overwrite confirm before extraction.
func (s *SftpService) ListArchiveConflicts(sessionID, archivePath, destDir string) ([]string, error) {
	return s.mgr.ListArchiveConflicts(sessionID, archivePath, destDir)
}

// ExtractRemoteArchive extracts a remote archive (zip, tar, tar.gz/tgz,
// tar.bz2, tar.xz or a bare .gz/.bz2/.xz stream) into destDir, stripping the
// archive's common root folder (extracts into the current directory).
func (s *SftpService) ExtractRemoteArchive(sessionID, archivePath, destDir string, overwrite bool) error {
	return s.mgr.ExtractRemoteArchive(sessionID, archivePath, destDir, overwrite)
}
