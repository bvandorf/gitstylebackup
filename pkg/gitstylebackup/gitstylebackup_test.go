package gitstylebackup

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEncryptionKeyDerivation tests password and key file encryption key generation
func TestEncryptionKeyDerivation(t *testing.T) {
	// Test password-based key derivation
	cfg1 := Config{
		EncryptPassword: "test123password",
	}
	
	key1, err := getEncryptionKey(cfg1)
	if err != nil {
		t.Fatalf("Failed to derive key from password: %v", err)
	}
	
	if len(key1) != 32 {
		t.Errorf("Expected 32-byte key, got %d bytes", len(key1))
	}
	
	// Test key file-based encryption
	tempKeyFile := filepath.Join(os.TempDir(), "test_key.key")
	defer os.Remove(tempKeyFile)
	
	err = ioutil.WriteFile(tempKeyFile, []byte("test-key-content-for-encryption"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}
	
	cfg2 := Config{
		EncryptKeyFile: tempKeyFile,
	}
	
	key2, err := getEncryptionKey(cfg2)
	if err != nil {
		t.Fatalf("Failed to derive key from file: %v", err)
	}
	
	if len(key2) != 32 {
		t.Errorf("Expected 32-byte key, got %d bytes", len(key2))
	}
	
	// Test no encryption
	cfg3 := Config{}
	key3, err := getEncryptionKey(cfg3)
	if err != nil {
		t.Fatalf("Failed to handle no encryption: %v", err)
	}
	
	if key3 != nil {
		t.Errorf("Expected nil key for no encryption, got %v", key3)
	}
}

// TestEncryptDecryptData tests the core encryption/decryption functionality
func TestEncryptDecryptData(t *testing.T) {
	key := deriveKey("test-password")
	originalData := []byte("This is test data for encryption testing.")
	
	// Test encryption
	encryptedData, err := encryptData(originalData, key)
	if err != nil {
		t.Fatalf("Failed to encrypt data: %v", err)
	}
	
	if len(encryptedData) <= len(originalData) {
		t.Errorf("Encrypted data should be longer than original due to nonce and auth tag")
	}
	
	// Test decryption
	decryptedData, err := decryptData(encryptedData, key)
	if err != nil {
		t.Fatalf("Failed to decrypt data: %v", err)
	}
	
	if string(decryptedData) != string(originalData) {
		t.Errorf("Decrypted data doesn't match original. Got: %s, Expected: %s", 
			string(decryptedData), string(originalData))
	}
}

// TestConfigWithEncryption tests config reading/writing with encryption fields
func TestConfigWithEncryption(t *testing.T) {
	tempConfigFile := filepath.Join(os.TempDir(), "test_config.json")
	defer os.Remove(tempConfigFile)
	
	originalConfig := Config{
		BackupDir:       "C:\\test\\backup",
		Include:         []string{"C:\\test\\source"},
		Exclude:         []string{"C:\\test\\exclude"},
		Priority:        "3",
		EncryptPassword: "test-password-123",
		RestoreStageDir: "C:\\test\\staging",
	}
	
	// Test writing config
	err := WriteConfig(tempConfigFile, originalConfig)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	
	// Test reading config
	readConfig, err := ReadConfig(tempConfigFile)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	
	// Verify all fields
	if readConfig.BackupDir != originalConfig.BackupDir {
		t.Errorf("BackupDir mismatch: got %s, expected %s", readConfig.BackupDir, originalConfig.BackupDir)
	}
	
	if readConfig.EncryptPassword != originalConfig.EncryptPassword {
		t.Errorf("EncryptPassword mismatch: got %s, expected %s", readConfig.EncryptPassword, originalConfig.EncryptPassword)
	}
	
	if readConfig.RestoreStageDir != originalConfig.RestoreStageDir {
		t.Errorf("RestoreStageDir mismatch: got %s, expected %s", readConfig.RestoreStageDir, originalConfig.RestoreStageDir)
	}
}

// TestRestoreStateManagement tests the restore state save/load functionality
func TestRestoreStateManagement(t *testing.T) {
	tempStateFile := filepath.Join(os.TempDir(), "test_restore_state.json")
	defer os.Remove(tempStateFile)
	
	originalState := RestoreState{
		Version:        1,
		BackupDir:      "C:\\test\\backup",
		RestoreDir:     "C:\\test\\restore",
		StageDir:       "C:\\test\\staging",
		Encrypted:      true,
		CopiedFiles:    []string{"hash1", "hash2", "hash3"},
		ExtractedFiles: []string{"file1.txt", "file2.txt"},
		Phase:          "extracting",
		StartTime:      "01/01/2025 12:00:00 -0500",
	}
	
	// Test saving state
	err := saveRestoreState(tempStateFile, originalState)
	if err != nil {
		t.Fatalf("Failed to save restore state: %v", err)
	}
	
	// Test loading state
	loadedState, err := loadRestoreState(tempStateFile)
	if err != nil {
		t.Fatalf("Failed to load restore state: %v", err)
	}
	
	// Verify critical fields
	if loadedState.Version != originalState.Version {
		t.Errorf("Version mismatch: got %d, expected %d", loadedState.Version, originalState.Version)
	}
	
	if loadedState.Phase != originalState.Phase {
		t.Errorf("Phase mismatch: got %s, expected %s", loadedState.Phase, originalState.Phase)
	}
	
	if len(loadedState.CopiedFiles) != len(originalState.CopiedFiles) {
		t.Errorf("CopiedFiles length mismatch: got %d, expected %d", 
			len(loadedState.CopiedFiles), len(originalState.CopiedFiles))
	}
	
	if loadedState.Encrypted != originalState.Encrypted {
		t.Errorf("Encrypted flag mismatch: got %t, expected %t", loadedState.Encrypted, originalState.Encrypted)
	}
}

// TestFileOperations tests basic file operations used by backup/restore
func TestFileOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "gitstyle_test")
	tempFile := filepath.Join(tempDir, "test.txt")
	testContent := "This is test content for file operations."
	
	// Clean up after test
	defer os.RemoveAll(tempDir)
	
	// Test directory creation
	err := MakeDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	exists, err := FolderExists(tempDir)
	if err != nil || !exists {
		t.Fatalf("Directory should exist after creation: exists=%t, err=%v", exists, err)
	}
	
	// Test file creation and writing
	err = ioutil.WriteFile(tempFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	// Test file existence check
	exists, err = FileExists(tempFile)
	if err != nil || !exists {
		t.Fatalf("File should exist after creation: exists=%t, err=%v", exists, err)
	}
	
	// Test file hashing
	hash, err := HashFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to hash file: %v", err)
	}
	
	hashString := HashToString(hash)
	if len(hashString) == 0 {
		t.Errorf("Hash string should not be empty")
	}
	
	// Test file size
	size := GetFileSize(tempFile)
	if size <= 0 {
		t.Errorf("File size should be positive, got %f", size)
	}
	
	// Test file deletion
	err = FileDelete(tempFile)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}
	
	exists, err = FileExists(tempFile)
	if err != nil || exists {
		t.Fatalf("File should not exist after deletion: exists=%t, err=%v", exists, err)
	}
}

// TestEncryptedFileOperations tests file compression and encryption together
func TestEncryptedFileOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "gitstyle_encrypt_test")
	sourceFile := filepath.Join(tempDir, "source.txt")
	encryptedFile := filepath.Join(tempDir, "encrypted.gz")
	decryptedFile := filepath.Join(tempDir, "decrypted.txt")
	testContent := "This is test content for encryption file operations testing."
	
	// Clean up after test
	defer os.RemoveAll(tempDir)
	
	// Setup
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	err = ioutil.WriteFile(sourceFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}
	
	// Test encryption with compression
	encryptionKey := deriveKey("test-encryption-key")
	err = CopyFileAndGZipWithEncryption(sourceFile, encryptedFile, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to encrypt and compress file: %v", err)
	}
	
	// Verify encrypted file exists and is different size
	exists, err := FileExists(encryptedFile)
	if err != nil || !exists {
		t.Fatalf("Encrypted file should exist: exists=%t, err=%v", exists, err)
	}
	
	// Test decryption and decompression
	err = ExtractGZipAndDecrypt(encryptedFile, decryptedFile, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to decrypt and decompress file: %v", err)
	}
	
	// Verify content matches
	decryptedContent, err := ioutil.ReadFile(decryptedFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}
	
	if string(decryptedContent) != testContent {
		t.Errorf("Decrypted content doesn't match original.\nGot: %s\nExpected: %s", 
			string(decryptedContent), testContent)
	}
}

// TestUnencryptedFileOperations tests file operations without encryption
func TestUnencryptedFileOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "gitstyle_unencrypt_test")
	sourceFile := filepath.Join(tempDir, "source.txt")
	compressedFile := filepath.Join(tempDir, "compressed.gz")
	decompressedFile := filepath.Join(tempDir, "decompressed.txt")
	testContent := "This is test content for unencrypted file operations testing."
	
	// Clean up after test
	defer os.RemoveAll(tempDir)
	
	// Setup
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	err = ioutil.WriteFile(sourceFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}
	
	// Test compression without encryption
	err = CopyFileAndGZip(sourceFile, compressedFile)
	if err != nil {
		t.Fatalf("Failed to compress file: %v", err)
	}
	
	// Test decompression without decryption
	err = ExtractGZipAndDecrypt(compressedFile, decompressedFile, nil)
	if err != nil {
		t.Fatalf("Failed to decompress file: %v", err)
	}
	
	// Verify content matches
	decompressedContent, err := ioutil.ReadFile(decompressedFile)
	if err != nil {
		t.Fatalf("Failed to read decompressed file: %v", err)
	}
	
	if string(decompressedContent) != testContent {
		t.Errorf("Decompressed content doesn't match original.\nGot: %s\nExpected: %s", 
			string(decompressedContent), testContent)
	}
}

// BenchmarkEncryption benchmarks the encryption performance
func BenchmarkEncryption(b *testing.B) {
	key := deriveKey("benchmark-password")
	data := []byte(strings.Repeat("This is benchmark data for encryption testing. ", 100))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptData(data, key)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}
}

// BenchmarkDecryption benchmarks the decryption performance
func BenchmarkDecryption(b *testing.B) {
	key := deriveKey("benchmark-password")
	data := []byte(strings.Repeat("This is benchmark data for decryption testing. ", 100))
	
	encryptedData, err := encryptData(data, key)
	if err != nil {
		b.Fatalf("Failed to encrypt data for benchmark: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decryptData(encryptedData, key)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}
	}
}
