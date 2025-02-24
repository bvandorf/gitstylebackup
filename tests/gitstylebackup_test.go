package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/bvandorf/gitstylebackup/pkg/gitstylebackup"
)

// TestConfig holds test configuration
type TestConfig struct {
	tempDir string
	cleanup func()
}

// setupTest creates a temporary test environment
func setupTest(t *testing.T) *TestConfig {
	tempDir, err := os.MkdirTemp("", "gitstylebackup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return &TestConfig{
		tempDir: tempDir,
		cleanup: func() {
			os.RemoveAll(tempDir)
		},
	}
}

func TestFileOperations(t *testing.T) {
	tc := setupTest(t)
	defer tc.cleanup()

	// Test MakeDir
	t.Run("MakeDir", func(t *testing.T) {
		testDir := filepath.Join(tc.tempDir, "testdir")
		if err := gitstylebackup.MakeDir(testDir); err != nil {
			t.Errorf("MakeDir failed: %v", err)
		}

		// Test creating existing directory
		if err := gitstylebackup.MakeDir(testDir); err == nil {
			t.Error("MakeDir should fail on existing directory")
		}
	})

	// Test FileExists
	t.Run("FileExists", func(t *testing.T) {
		testFile := filepath.Join(tc.tempDir, "testfile")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		exists, err := gitstylebackup.FileExists(testFile)
		if !exists || err != nil {
			t.Errorf("FileExists failed for existing file: exists=%v, err=%v", exists, err)
		}

		exists, err = gitstylebackup.FileExists(filepath.Join(tc.tempDir, "nonexistent"))
		if exists || err != nil {
			t.Errorf("FileExists should return false for non-existent file: exists=%v, err=%v", exists, err)
		}
	})

	// Test FolderExists
	t.Run("FolderExists", func(t *testing.T) {
		testDir := filepath.Join(tc.tempDir, "testdir2")
		if err := os.Mkdir(testDir, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		exists, err := gitstylebackup.FolderExists(testDir)
		if !exists || err != nil {
			t.Errorf("FolderExists failed for existing directory: exists=%v, err=%v", exists, err)
		}

		exists, err = gitstylebackup.FolderExists(filepath.Join(tc.tempDir, "nonexistent"))
		if exists || err != nil {
			t.Errorf("FolderExists should return false for non-existent directory: exists=%v, err=%v", exists, err)
		}
	})

	// Test FileDelete
	t.Run("FileDelete", func(t *testing.T) {
		testFile := filepath.Join(tc.tempDir, "testfile2")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		if err := gitstylebackup.FileDelete(testFile); err != nil {
			t.Errorf("FileDelete failed: %v", err)
		}

		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File should not exist after deletion")
		}
	})

	// Test Config operations
	t.Run("Config", func(t *testing.T) {
		cfg := gitstylebackup.Config{
			BackupDir: tc.tempDir,
			Include:   []string{"path1", "path2"},
			Exclude:   []string{"exclude1"},
		}

		configFile := filepath.Join(tc.tempDir, "config.json")
		if err := gitstylebackup.WriteConfig(configFile, cfg); err != nil {
			t.Errorf("WriteConfig failed: %v", err)
		}

		readCfg, err := gitstylebackup.ReadConfig(configFile)
		if err != nil {
			t.Errorf("ReadConfig failed: %v", err)
		}

		if readCfg.BackupDir != cfg.BackupDir {
			t.Errorf("Config BackupDir mismatch: got %s, want %s", readCfg.BackupDir, cfg.BackupDir)
		}
	})

	// Test hash functions
	t.Run("HashOperations", func(t *testing.T) {
		testFile := filepath.Join(tc.tempDir, "hashtest")
		testData := []byte("test data for hashing")
		if err := os.WriteFile(testFile, testData, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		hash, err := gitstylebackup.HashFile(testFile)
		if err != nil {
			t.Errorf("HashFile failed: %v", err)
		}
		if len(hash) == 0 {
			t.Error("HashFile returned empty hash")
		}

		hashStr := gitstylebackup.HashToString(hash)
		if len(hashStr) == 0 {
			t.Error("HashToString returned empty string")
		}
	})

	// Test file utilities
	t.Run("FileUtilities", func(t *testing.T) {
		testFile := filepath.Join(tc.tempDir, "sizetest")
		testData := []byte("test data for size check")
		if err := os.WriteFile(testFile, testData, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		size := gitstylebackup.GetFileSize(testFile)
		if size <= 0 {
			t.Errorf("GetFileSize returned invalid size: %f", size)
		}

		modTime := gitstylebackup.GetFileModifiedDate(testFile)
		if modTime.IsZero() {
			t.Error("GetFileModifiedDate returned zero time")
		}
	})

	// Test CopyFileAndGZip
	t.Run("CopyFileAndGZip", func(t *testing.T) {
		srcFile := filepath.Join(tc.tempDir, "source")
		dstFile := filepath.Join(tc.tempDir, "destination")
		testData := []byte("test data for compression")

		if err := os.WriteFile(srcFile, testData, 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		if err := gitstylebackup.CopyFileAndGZip(srcFile, dstFile); err != nil {
			t.Errorf("CopyFileAndGZip failed: %v", err)
		}

		// Verify destination file exists
		if _, err := os.Stat(dstFile); os.IsNotExist(err) {
			t.Error("Destination file was not created")
		}
	})
}

func TestBackupIntegration(t *testing.T) {
	tc := setupTest(t)
	defer tc.cleanup()

	// Create test files and directories
	sourceDir := filepath.Join(tc.tempDir, "source")
	backupDir := filepath.Join(tc.tempDir, "backup")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	// Create test files
	testFiles := []struct {
		name    string
		content string
	}{
		{"file1.txt", "test content 1"},
		{"file2.txt", "test content 2"},
		{"subdir/file3.txt", "test content 3"},
	}

	for _, tf := range testFiles {
		path := filepath.Join(sourceDir, tf.name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", tf.name, err)
		}
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	// Create test config
	cfg := gitstylebackup.Config{
		BackupDir: backupDir,
		Include:   []string{sourceDir},
		Exclude:   []string{filepath.Join(sourceDir, "subdir")},
	}

	// Run backup
	if err := gitstylebackup.Backup(cfg); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify backup structure
	t.Run("BackupStructure", func(t *testing.T) {
		versionDir := filepath.Join(backupDir, "Version")
		filesDir := filepath.Join(backupDir, "Files")

		if _, err := os.Stat(versionDir); os.IsNotExist(err) {
			t.Error("Version folder was not created")
		}
		if _, err := os.Stat(filesDir); os.IsNotExist(err) {
			t.Error("Files folder was not created")
		}
	})

	// Test trim functionality
	t.Run("TrimOperation", func(t *testing.T) {
		if err := gitstylebackup.Trim(cfg, "1"); err != nil {
			t.Errorf("Trim failed: %v", err)
		}
	})

	// Test verify functionality
	t.Run("VerifyOperation", func(t *testing.T) {
		if err := gitstylebackup.Verify(cfg, "1"); err != nil {
			t.Errorf("Verify failed: %v", err)
		}
	})
}

func TestSymlinkHandling(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping symlink test on non-Windows platform")
	}

	tc := setupTest(t)
	defer tc.cleanup()

	// Create a directory with a file
	sourceDir := filepath.Join(tc.tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	testFile := filepath.Join(sourceDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a symlink that points to the parent directory (would cause recursion)
	symlinkPath := filepath.Join(sourceDir, "recursive")
	if err := os.Symlink(sourceDir, symlinkPath); err != nil {
		// If symlink creation fails, just log and continue
		t.Logf("Failed to create symlink (might need admin rights): %v", err)
		return
	}

	// Run backup with the test directory
	cfg := gitstylebackup.Config{
		BackupDir: filepath.Join(tc.tempDir, "backup"),
		Include:   []string{sourceDir},
	}

	// This should complete without infinite recursion
	if err := gitstylebackup.Backup(cfg); err != nil {
		t.Errorf("Backup with symlink failed: %v", err)
	}
}

func TestErrorHandling(t *testing.T) {
	tc := setupTest(t)
	defer tc.cleanup()

	t.Run("InvalidConfig", func(t *testing.T) {
		cfg := gitstylebackup.Config{
			BackupDir: "", // Invalid empty directory
			Include:   []string{},
			Exclude:   []string{},
		}
		if err := gitstylebackup.Backup(cfg); err == nil {
			t.Error("Expected error for invalid config")
		}
	})

	t.Run("NonExistentSourceDir", func(t *testing.T) {
		cfg := gitstylebackup.Config{
			BackupDir: tc.tempDir,
			Include:   []string{"non/existent/path"},
			Exclude:   []string{},
		}
		if err := gitstylebackup.Backup(cfg); err == nil {
			t.Error("Expected error for non-existent source directory")
		}
	})

	t.Run("InvalidTrimVersion", func(t *testing.T) {
		cfg := gitstylebackup.Config{
			BackupDir: tc.tempDir,
			Include:   []string{tc.tempDir},
		}
		if err := gitstylebackup.Trim(cfg, "invalid"); err == nil {
			t.Error("Expected error for invalid trim version")
		}
	})

	t.Run("InvalidVerifyVersion", func(t *testing.T) {
		cfg := gitstylebackup.Config{
			BackupDir: tc.tempDir,
			Include:   []string{tc.tempDir},
		}
		if err := gitstylebackup.Verify(cfg, "invalid"); err == nil {
			t.Error("Expected error for invalid verify version")
		}
	})
}

func TestFixOperations(t *testing.T) {
	tc := setupTest(t)
	defer tc.cleanup()

	// Create a test backup first
	sourceDir := filepath.Join(tc.tempDir, "source")
	backupDir := filepath.Join(tc.tempDir, "backup")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create test files
	testFile := filepath.Join(sourceDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := gitstylebackup.Config{
		BackupDir: backupDir,
		Include:   []string{sourceDir},
	}

	// Run backup
	if err := gitstylebackup.Backup(cfg); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	t.Run("Fix", func(t *testing.T) {
		// Create some orphaned files in the backup directory
		orphanDir := filepath.Join(backupDir, "Files", "00")
		if err := os.MkdirAll(orphanDir, 0755); err != nil {
			t.Fatalf("Failed to create orphan directory: %v", err)
		}
		orphanFile := filepath.Join(orphanDir, "orphan")
		if err := os.WriteFile(orphanFile, []byte("orphan"), 0644); err != nil {
			t.Fatalf("Failed to create orphan file: %v", err)
		}

		// Run fix
		if err := gitstylebackup.Fix(cfg); err != nil {
			t.Errorf("Fix failed: %v", err)
		}

		// Verify orphan file was removed
		if _, err := os.Stat(orphanFile); !os.IsNotExist(err) {
			t.Error("Orphan file should have been removed")
		}
	})

	t.Run("FixInUse", func(t *testing.T) {
		// Create an InUse file
		inUseFile := filepath.Join(backupDir, "InUse.txt")
		if err := os.WriteFile(inUseFile, []byte{}, 0644); err != nil {
			t.Fatalf("Failed to create InUse file: %v", err)
		}

		// Run FixInUse
		if err := gitstylebackup.FixInUse(cfg); err != nil {
			t.Errorf("FixInUse failed: %v", err)
		}

		// Verify InUse file was removed
		if _, err := os.Stat(inUseFile); !os.IsNotExist(err) {
			t.Error("InUse file should have been removed")
		}
	})
}

func TestConcurrentOperations(t *testing.T) {
	tc := setupTest(t)
	defer tc.cleanup()

	sourceDir := filepath.Join(tc.tempDir, "source")
	backupDir := filepath.Join(tc.tempDir, "backup")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create multiple test files
	for i := 0; i < 100; i++ {
		testFile := filepath.Join(sourceDir, fmt.Sprintf("test%d.txt", i))
		if err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	cfg := gitstylebackup.Config{
		BackupDir: backupDir,
		Include:   []string{sourceDir},
	}

	// Run multiple backups concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := gitstylebackup.Backup(cfg)
			if err != nil && !strings.Contains(err.Error(), "backup directory is in use") {
				errChan <- fmt.Errorf("Unexpected error during concurrent backup: %v", err)
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		t.Error(err)
	}

	// Verify backup was successful
	versionDir := filepath.Join(backupDir, "Version")
	filesDir := filepath.Join(backupDir, "Files")

	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		t.Error("Version folder was not created")
	}
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		t.Error("Files folder was not created")
	}
}
