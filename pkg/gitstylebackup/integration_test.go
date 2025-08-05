package gitstylebackup

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestFullBackupRestoreWorkflow tests the complete backup and restore process
func TestFullBackupRestoreWorkflow(t *testing.T) {
	// Setup test environment
	tempDir := filepath.Join(os.TempDir(), "gitstyle_integration_test")
	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backup")
	restoreDir := filepath.Join(tempDir, "restore")
	
	// Clean up after test
	defer os.RemoveAll(tempDir)
	
	// Create test directory structure
	err := os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	
	subDir := filepath.Join(sourceDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	
	// Create test files
	testFiles := map[string]string{
		filepath.Join(sourceDir, "file1.txt"):        "Content of file 1 for integration testing.",
		filepath.Join(sourceDir, "file2.txt"):        "Content of file 2 with different data.",
		filepath.Join(subDir, "file3.txt"):           "Content of file 3 in subdirectory.",
	}
	
	for filePath, content := range testFiles {
		err = ioutil.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}
	
	// Create config for backup
	config := Config{
		BackupDir: backupDir,
		Include:   []string{sourceDir},
		Exclude:   []string{},
		Priority:  "3",
	}
	
	// Test backup without encryption
	err = Backup(config)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	
	// Verify backup structure was created
	versionDir := filepath.Join(backupDir, "version")
	filesDir := filepath.Join(backupDir, "files")
	
	exists, err := FolderExists(versionDir)
	if err != nil || !exists {
		t.Fatalf("Version directory should exist: exists=%t, err=%v", exists, err)
	}
	
	exists, err = FolderExists(filesDir)
	if err != nil || !exists {
		t.Fatalf("Files directory should exist: exists=%t, err=%v", exists, err)
	}
	
	// Test restore
	err = Restore(config, "1", restoreDir)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	
	// Verify restored files
	for originalPath, expectedContent := range testFiles {
		// Calculate relative path for restored file
		relPath, err := filepath.Rel(sourceDir, originalPath)
		if err != nil {
			t.Fatalf("Failed to calculate relative path: %v", err)
		}
		
		restoredPath := filepath.Join(restoreDir, relPath)
		
		// Check if restored file exists
		exists, err := FileExists(restoredPath)
		if err != nil || !exists {
			t.Fatalf("Restored file should exist %s: exists=%t, err=%v", restoredPath, exists, err)
		}
		
		// Check content
		content, err := ioutil.ReadFile(restoredPath)
		if err != nil {
			t.Fatalf("Failed to read restored file %s: %v", restoredPath, err)
		}
		
		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s.\nGot: %s\nExpected: %s", 
				restoredPath, string(content), expectedContent)
		}
	}
	
	// Verify no temporary files remain
	stateFile := filepath.Join(restoreDir, "restore_state.json")
	exists, err = FileExists(stateFile)
	if err == nil && exists {
		t.Errorf("State file should be cleaned up after restore: %s", stateFile)
	}
}

// TestEncryptedBackupRestoreWorkflow tests backup and restore with encryption
func TestEncryptedBackupRestoreWorkflow(t *testing.T) {
	// Setup test environment
	tempDir := filepath.Join(os.TempDir(), "gitstyle_encrypted_integration_test")
	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backup")
	restoreDir := filepath.Join(tempDir, "restore")
	
	// Clean up after test
	defer os.RemoveAll(tempDir)
	
	// Create test directory structure
	err := os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	
	// Create test file
	testFile := filepath.Join(sourceDir, "encrypted_test.txt")
	testContent := "This is content that will be encrypted during backup."
	err = ioutil.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create config with encryption
	config := Config{
		BackupDir:       backupDir,
		Include:         []string{sourceDir},
		Exclude:         []string{},
		Priority:        "3",
		EncryptPassword: "integration-test-password",
	}
	
	// Test encrypted backup
	err = Backup(config)
	if err != nil {
		t.Fatalf("Encrypted backup failed: %v", err)
	}
	
	// Test encrypted restore
	err = Restore(config, "1", restoreDir)
	if err != nil {
		t.Fatalf("Encrypted restore failed: %v", err)
	}
	
	// Verify restored content
	restoredFile := filepath.Join(restoreDir, "encrypted_test.txt")
	content, err := ioutil.ReadFile(restoredFile)
	if err != nil {
		t.Fatalf("Failed to read restored encrypted file: %v", err)
	}
	
	if string(content) != testContent {
		t.Errorf("Encrypted content mismatch.\nGot: %s\nExpected: %s", 
			string(content), testContent)
	}
}

// TestStagingRestoreWorkflow tests restore with separate staging directory
func TestStagingRestoreWorkflow(t *testing.T) {
	// Setup test environment
	tempDir := filepath.Join(os.TempDir(), "gitstyle_staging_integration_test")
	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backup")
	restoreDir := filepath.Join(tempDir, "restore")
	stageDir := filepath.Join(tempDir, "staging")
	
	// Clean up after test
	defer os.RemoveAll(tempDir)
	
	// Create test directory structure
	err := os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	
	// Create test file
	testFile := filepath.Join(sourceDir, "staging_test.txt")
	testContent := "This is content for testing staging restore workflow."
	err = ioutil.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create config with staging directory
	config := Config{
		BackupDir:       backupDir,
		Include:         []string{sourceDir},
		Exclude:         []string{},
		Priority:        "3",
		RestoreStageDir: stageDir,
	}
	
	// Test backup
	err = Backup(config)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	
	// Test restore with staging
	err = Restore(config, "1", restoreDir)
	if err != nil {
		t.Fatalf("Staging restore failed: %v", err)
	}
	
	// Verify restored content
	restoredFile := filepath.Join(restoreDir, "staging_test.txt")
	content, err := ioutil.ReadFile(restoredFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}
	
	if string(content) != testContent {
		t.Errorf("Staging restore content mismatch.\nGot: %s\nExpected: %s", 
			string(content), testContent)
	}
	
	// Verify staging directory was cleaned up
	exists, err := FolderExists(stageDir)
	if err == nil && exists {
		// Check if staging directory is empty (it might exist but be empty)
		files, err := ioutil.ReadDir(stageDir)
		if err == nil && len(files) > 0 {
			t.Errorf("Staging directory should be cleaned up or empty after restore")
		}
	}
}

// TestMultipleVersionsWorkflow tests backup and restore with multiple versions
func TestMultipleVersionsWorkflow(t *testing.T) {
	// Setup test environment
	tempDir := filepath.Join(os.TempDir(), "gitstyle_versions_integration_test")
	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backup")
	restoreDir := filepath.Join(tempDir, "restore")
	
	// Clean up after test
	defer os.RemoveAll(tempDir)
	
	// Create test directory structure
	err := os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	
	config := Config{
		BackupDir: backupDir,
		Include:   []string{sourceDir},
		Exclude:   []string{},
		Priority:  "3",
	}
	
	// Create multiple backup versions
	versions := []string{
		"Version 1 content for testing multiple backups.",
		"Version 2 content with different data for testing.",
		"Version 3 content with yet more different data.",
	}
	
	testFile := filepath.Join(sourceDir, "version_test.txt")
	
	for i, content := range versions {
		// Update file content
		err = ioutil.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file for version %d: %v", i+1, err)
		}
		
		// Create backup
		err = Backup(config)
		if err != nil {
			t.Fatalf("Backup %d failed: %v", i+1, err)
		}
	}
	
	// Test restoring each version
	for i, expectedContent := range versions {
		versionNum := strconv.Itoa(i + 1)
		
		// Clean restore directory
		os.RemoveAll(restoreDir)
		
		// Restore specific version
		err = Restore(config, versionNum, restoreDir)
		if err != nil {
			t.Fatalf("Restore version %s failed: %v", versionNum, err)
		}
		
		// Verify content
		restoredFile := filepath.Join(restoreDir, "version_test.txt")
		content, err := ioutil.ReadFile(restoredFile)
		if err != nil {
			t.Fatalf("Failed to read restored file for version %s: %v", versionNum, err)
		}
		
		if string(content) != expectedContent {
			t.Errorf("Version %s content mismatch.\nGot: %s\nExpected: %s", 
				versionNum, string(content), expectedContent)
		}
	}
}

// TestErrorHandling tests various error conditions
func TestErrorHandling(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "gitstyle_error_test")
	defer os.RemoveAll(tempDir)
	
	// Test restore with non-existent version
	config := Config{
		BackupDir: filepath.Join(tempDir, "nonexistent"),
		Include:   []string{tempDir},
		Exclude:   []string{},
		Priority:  "3",
	}
	
	err := Restore(config, "999", filepath.Join(tempDir, "restore"))
	if err == nil {
		t.Errorf("Restore should fail with non-existent backup version")
	}
	
	// Test with invalid encryption key file
	config.EncryptKeyFile = filepath.Join(tempDir, "nonexistent.key")
	_, err = getEncryptionKey(config)
	if err == nil {
		t.Errorf("Should fail with non-existent key file")
	}
}
