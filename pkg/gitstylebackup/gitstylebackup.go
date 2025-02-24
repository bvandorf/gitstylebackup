// Copyright 2016 By Brad Van Dorf All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// Author: Brad Van Dorf (github.com/bvandorf)

package gitstylebackup

import (
	"bufio"
	"compress/gzip"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

var usageStr = `
Backup Options:
-b, --backup                Use to backup using config file
-t, --trim <version>        Use to trim backup directory to version's specified
           <+x>             Use to trim backup directory to keep current + x version's specified
-v, --verify <version>      Use to verify files in backup directory current version is 0 
-c, --config <file>         Use to specify the config file used (default: config.txt)
    --exampleconfig <file>  Use to make an example config file
    --fix                   Use to fix interrupted backup or trim
    --fixinuse              Use to remove inuse flag from backup

Common Options:
-h, --help                  Show this help
    --version               Show version

Notes:
case is important when defining paths in the config file

Exit Codes:
     0 = Clean
    -1 = Version or help
     1 = Error
`

const timeFormat = "01/02/2006 15:04:05 -0700"
const fileNewLine = "\r\n"

func usage() {
	fmt.Printf("%s\n", usageStr)
	os.Exit(-1)
}

// Config holds the backup configuration
type Config struct {
	BackupDir   string   `json:"backupDir"`
	Include     []string `json:"include"`
	Exclude     []string `json:"exclude"`
	trimValue   string   `json:"-"`
	verifyValue string   `json:"-"`
}

var dbBackupFolder = ""
var dbBackupVersionFolder = ""
var dbBackupFilesFolder = ""
var dbBackupInUseFile = ""

func main() {
	var showHelp bool
	flag.BoolVar(&showHelp, "h", false, "")
	flag.BoolVar(&showHelp, "help", false, "")

	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "")

	var configFilePath string
	flag.StringVar(&configFilePath, "c", "./config.txt", "")
	flag.StringVar(&configFilePath, "config", "./config.txt", "")

	var exampleConfig string
	flag.StringVar(&exampleConfig, "exampleconfig", "", "")

	var runBackup bool
	flag.BoolVar(&runBackup, "b", false, "")
	flag.BoolVar(&runBackup, "backup", false, "")

	var runTrim bool
	var trimVersionArg = ""
	flag.StringVar(&trimVersionArg, "t", "", "")
	flag.StringVar(&trimVersionArg, "trim", "", "")

	var runFix bool
	flag.BoolVar(&runFix, "fix", false, "")

	var runFixInuse bool
	flag.BoolVar(&runFixInuse, "fixinuse", false, "")

	var runVerify bool
	var verifyVersionArg = ""
	flag.StringVar(&verifyVersionArg, "v", "", "")
	flag.StringVar(&verifyVersionArg, "verify", "", "")

	flag.Usage = usage
	flag.Parse()

	if trimVersionArg != "" {
		runTrim = true
	}

	if verifyVersionArg != "" {
		runVerify = true
	}

	if showHelp {
		usage()
	}

	if showVersion {
		fmt.Println("Version 1.3")
		os.Exit(-1)
	}

	var iCheckArgs = 0
	if runBackup {
		iCheckArgs++
	}
	if runTrim {
		iCheckArgs++
	}
	if runFix {
		iCheckArgs++
	}
	if runFixInuse {
		iCheckArgs++
	}
	if runVerify {
		iCheckArgs++
	}
	if exampleConfig != "" {
		iCheckArgs++
	}
	if iCheckArgs > 1 {
		fmt.Println("You Cant Use All Arguments At The Same Time")
		usage()
	}
	if iCheckArgs == 0 {
		usage()
	}

	if exampleConfig != "" {
		var eConfig = Config{}
		eConfig.BackupDir = "C:\\Temp"
		eConfig.Include = append(eConfig.Include, "C:\\Users")
		eConfig.Include = append(eConfig.Include, "C:\\ProgramData")
		eConfig.Exclude = append(eConfig.Exclude, "C:\\Users\\Default")

		if err := WriteConfig(exampleConfig, eConfig); err != nil {
			fmt.Println("Error Writing Example Config File: " + err.Error())
			os.Exit(1)
		}

		os.Exit(0)
	}

	cfg, err := ReadConfig(configFilePath)
	if err != nil {
		fmt.Println("Error Reading Config File: " + err.Error())
		os.Exit(1)
	}

	dbBackupFolder = strings.TrimRight(cfg.BackupDir, "\\")
	dbBackupVersionFolder = dbBackupFolder + "\\Version"
	dbBackupFilesFolder = dbBackupFolder + "\\Files"
	dbBackupInUseFile = dbBackupFolder + "\\InUse.txt"

	//check if backup dir in use
	exists, err := FileExists(dbBackupInUseFile)
	if exists || err != nil {
		if err != nil {
			fmt.Println("In Use File Exists " + err.Error())
			os.Exit(1)
		} else {
			fmt.Println("In Use File Exists ")
			os.Exit(1)
		}
	}

	//mark backup folder in use
	err = WriteByteSliceToFile(dbBackupInUseFile, []byte{})
	if err != nil {
		fmt.Println("Writeing In Use File " + err.Error())
		os.Exit(1)
	}

	if runBackup {
		BackupFiles(cfg)
	}

	if runTrim {
		cfg.trimValue = trimVersionArg
		TrimFiles(cfg)
	}

	if runFix {
		FixFiles(cfg)
	}

	if runFixInuse {
		FixFileInUse(cfg)
	}

	if runVerify {
		cfg.verifyValue = verifyVersionArg
		VerifyFiles(cfg)
	}

	//remove in use file
	err = FileDelete(dbBackupInUseFile)
	if err != nil {
		fmt.Println("Deleting In Use File " + err.Error())
		os.Exit(1)
	}
}

func BackupFiles(cfg Config) error {
	//make sure dir is setup
	exists, err := FolderExists(dbBackupVersionFolder)
	if exists == false && err == nil {
		err = MakeDir(dbBackupVersionFolder)
		if err != nil {
			return fmt.Errorf("error making version folder: %v", err)
		}
	}

	exists, err = FolderExists(dbBackupFilesFolder)
	if exists == false && err == nil {
		err = MakeDir(dbBackupFilesFolder)
		if err != nil {
			return fmt.Errorf("error making files folder: %v", err)
		}

		for i := 0; i <= 25; i++ {
			err = MakeDir(dbBackupFilesFolder + "\\" + fmt.Sprintf("%02d", i))
			if err != nil {
				return fmt.Errorf("error making subfiles folder: %v", err)
			}
		}
	}

	//find max version number
	var dbNewVersionNumber = 0
	verDirFile, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		return fmt.Errorf("error reading version files: %v", err)
	}
	for _, verDF := range verDirFile {
		if verDF.IsDir() == false {
			if strings.HasSuffix(verDF.Name(), ".tmp") {
				err = FileDelete(dbBackupVersionFolder + "\\" + verDF.Name())
				if err != nil {
					return fmt.Errorf("error cleaning up temp version %s: %v", verDF.Name(), err)
				}
			} else {
				testVer, err := strconv.Atoi(verDF.Name())
				if err != nil {
					return fmt.Errorf("error parsing version file: %v", err)
				}

				if dbNewVersionNumber < testVer {
					dbNewVersionNumber = testVer
				}
			}
		}
	}

	dbNewVersionNumber = dbNewVersionNumber + 1

	var dbBackupNewVersionFile = dbBackupVersionFolder + "\\" + strconv.Itoa(dbNewVersionNumber)
	var dbBackupNewTempVersionFile = dbBackupNewVersionFile + ".tmp"

	//open version file
	verFile, err := os.OpenFile(dbBackupNewTempVersionFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening version file: %v", err)
	}
	defer verFile.Close()

	_, err = verFile.WriteString("VERSION:" + strconv.Itoa(dbNewVersionNumber) + fileNewLine +
		"DATE:" + time.Now().Format(timeFormat) + fileNewLine)
	if err != nil {
		return fmt.Errorf("error writing version file: %v", err)
	}

	walkedFiles := make(chan string)

	go func(t_walkFilePaths []string, t_walkFilePathsExclude []string, t_walkedFilesChan chan string) {
		for _, cd := range t_walkFilePaths {
			errc := filepath.Walk(cd, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Printf("Error accessing path %s: %v\n", path, err)
					return filepath.SkipDir // Skip this directory but continue walking
				}

				// Skip symlinks and non-regular files
				if info.Mode()&os.ModeSymlink != 0 {
					fmt.Printf("Skipping symlink: %s\n", path)
					return filepath.SkipDir
				}

				if !info.Mode().IsRegular() {
					return nil
				}

				// Check exclusions
				for _, ex := range t_walkFilePathsExclude {
					if strings.HasPrefix(strings.ToLower(filepath.Join(path, info.Name())), strings.ToLower(ex)) {
						return nil
					}
				}

				t_walkedFilesChan <- path
				return nil
			})

			if errc != nil {
				fmt.Printf("Warning: Error walking path %s: %v\n", cd, errc)
				// Continue with next path instead of exiting
			}
		}

		close(t_walkedFilesChan)
	}(cfg.Include, cfg.Exclude, walkedFiles)

	var wg sync.WaitGroup
	wg.Add(20)
	for i := 0; i < 20; i++ {
		go func() {
			for path := range walkedFiles {
				hash, err := HashFile(path)
				if err != nil {
					fmt.Printf("Warning: Error hashing file %s: %v\n", path, err)
					continue // Skip this file but continue processing
				}

				sFileHash := HashToString(hash)

				_, err = verFile.WriteString("FILE:" + path + fileNewLine +
					"MODDATE:" + GetFileModifiedDate(path).Format(timeFormat) + fileNewLine +
					"SIZE:" + strconv.FormatFloat(GetFileSize(path), 'f', 6, 64) + fileNewLine +
					"HASH:" + sFileHash + fileNewLine)
				if err != nil {
					fmt.Printf("Warning: Error writing to version file for %s: %v\n", path, err)
					continue // Skip this file but continue processing
				}

				exists, err := FileExists(dbBackupFilesFolder + "\\" + sFileHash[:2] + "\\" + sFileHash)
				if exists == false && err == nil {
					fmt.Println("COPYING FILE:" + path + " -> " + sFileHash)
					err := CopyFileAndGZip(path, dbBackupFilesFolder+"\\"+sFileHash[:2]+"\\"+sFileHash)
					if err != nil {
						fmt.Printf("Warning: Error copying file %s: %v\n", path, err)
						// Continue processing other files
					}
				} else if exists && err == nil {
					fmt.Println("SKIP FILE COPY:" + path + " -> " + sFileHash)
				} else {
					fmt.Printf("Warning: Error checking file existence %s: %v\n", path, err)
					// Continue processing other files
				}
			}

			wg.Done()
		}()
	}

	wg.Wait()

	// Make sure to close the file before renaming
	verFile.Close()

	err = os.Rename(dbBackupNewTempVersionFile, dbBackupNewVersionFile)
	if err != nil {
		return fmt.Errorf("error renaming version file: %v", err)
	}

	return nil
}

func TrimFiles(cfg Config) {

	exists, err := FolderExists(dbBackupVersionFolder)
	if exists == false || err != nil {
		if err != nil {
			fmt.Println("No Version Folder Found " + err.Error())
			os.Exit(1)
		} else {
			fmt.Println("No Version Folder Found")
			os.Exit(1)
		}
	}

	exists, err = FolderExists(dbBackupFilesFolder)
	if exists == false || err != nil {
		if err != nil {
			fmt.Println("No Files Folder Found " + err.Error())
			os.Exit(1)
		} else {
			fmt.Println("No Files Folder Found")
			os.Exit(1)
		}
	}

	//find max version number
	var dbMaxVersionNumber = 0
	verDirFile, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		fmt.Println("Error Reading Version Files " + err.Error())
		os.Exit(1)
	}
	for _, verDF := range verDirFile {
		if verDF.IsDir() == false {
			if strings.HasSuffix(verDF.Name(), ".tmp") {
				err = FileDelete(dbBackupVersionFolder + "\\" + verDF.Name())
				if err != nil {
					fmt.Println("Error Cleaning Up Temp Version " + verDF.Name() + " " + err.Error())
					os.Exit(1)
				}
			} else {
				testVer, err := strconv.Atoi(verDF.Name())
				if err != nil {
					fmt.Println("Error Parsing Version File " + err.Error())
					os.Exit(1)
				}

				if dbMaxVersionNumber < testVer {
					dbMaxVersionNumber = testVer
				}
			}
		}
	}

	//find what version to trim to
	trimVersion, err := strconv.Atoi(cfg.trimValue)
	if err != nil {
		fmt.Println("Error Parsing Trim Version")
		os.Exit(1)
	}

	if strings.Contains(cfg.trimValue, "+") {
		trimVersion = dbMaxVersionNumber - trimVersion
	}
	if trimVersion < 0 {
		trimVersion = 0
	}

	fmt.Println("Trimming To Version ", trimVersion)

	verFiles, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		fmt.Println("Error Reading Version Folder " + err.Error())
		os.Exit(1)
	}

	var toDel = map[string]bool{}
	for _, verDF := range verFiles {
		fmt.Println("Loading Version File " + verDF.Name())
		testVer, err := strconv.Atoi(verDF.Name())
		if err != nil {
			fmt.Println("Error Parsing Version File " + err.Error())
			os.Exit(1)
		}
		if testVer < trimVersion {
			verFile, err := os.Open(dbBackupVersionFolder + "\\" + verDF.Name())
			if err != nil {
				fmt.Println("Error Opening Version File " + verDF.Name() + " " + err.Error())
				os.Exit(1)
			}

			var verFileHash = ""
			scanner := bufio.NewScanner(verFile)
			for scanner.Scan() {
				verFileHash = scanner.Text()
				if strings.HasPrefix(verFileHash, "HASH:") {
					fmt.Println("Adding File Hash " + verFileHash[5:])
					toDel[verFileHash[5:]] = true
				}
			}

			verFile.Close()
		}
	}

	for _, verDF := range verFiles {
		fmt.Println("Comparing To Version File " + verDF.Name())
		testVer, err := strconv.Atoi(verDF.Name())
		if err != nil {
			fmt.Println("Error Parsing Version File " + err.Error())
			os.Exit(1)
		}
		if testVer >= trimVersion {
			verFile, err := os.Open(dbBackupVersionFolder + "\\" + verDF.Name())
			if err != nil {
				fmt.Println("Error Opening Version File " + verDF.Name() + " " + err.Error())
				os.Exit(1)
			}

			var verFileHash = ""
			scanner := bufio.NewScanner(verFile)
			for scanner.Scan() {
				verFileHash = scanner.Text()
				if strings.HasPrefix(verFileHash, "HASH:") {
					fmt.Println("Removeing File Hash " + verFileHash[5:])
					toDel[verFileHash[5:]] = false
				}
			}

			verFile.Close()
		}
	}

	//delete files from disk
	for key, val := range toDel {
		if val == true {
			fmt.Println("Deleting File " + key)
			err := FileDelete(dbBackupFilesFolder + "\\" + key[:2] + "\\" + key)
			if err != nil {
				fmt.Println("Error Deleting File " + key + " " + err.Error())
			}
		}
	}

	//delete version file from disk
	for ver := 1; ver < trimVersion; ver++ {
		exists, err = FileExists(dbBackupVersionFolder + "\\" + strconv.Itoa(ver))
		if exists && err == nil {
			fmt.Println("Deleteing Version ", ver)
			err = FileDelete(dbBackupVersionFolder + "\\" + strconv.Itoa(ver))
			if err != nil {
				fmt.Println("Error Deleteing Versin File " + strconv.Itoa(ver) + " " + err.Error())
			}
		} else if err != nil {
			fmt.Println("Error Deleteing Version File " + strconv.Itoa(ver) + " " + err.Error())
		}
	}

	return
}

func VerifyFiles(cfg Config) {

	exists, err := FolderExists(dbBackupVersionFolder)
	if exists == false || err != nil {
		if err != nil {
			fmt.Println("No Version Folder Found " + err.Error())
			os.Exit(1)
		} else {
			fmt.Println("No Version Folder Found")
			os.Exit(1)
		}
	}

	exists, err = FolderExists(dbBackupFilesFolder)
	if exists == false || err != nil {
		if err != nil {
			fmt.Println("No Files Folder Found " + err.Error())
			os.Exit(1)
		} else {
			fmt.Println("No Files Folder Found")
			os.Exit(1)
		}
	}

	//find what version to verify
	var verifyVersion = 0
	if cfg.verifyValue == "0" {
		//find max version number
		var dbMaxVersionNumber = 0
		verDirFile, err := ioutil.ReadDir(dbBackupVersionFolder)
		if err != nil {
			fmt.Println("Error Reading Version Files " + err.Error())
			os.Exit(1)
		}
		for _, verDF := range verDirFile {
			if verDF.IsDir() == false {
				if !strings.HasSuffix(verDF.Name(), ".tmp") {
					testVer, err := strconv.Atoi(verDF.Name())
					if err != nil {
						fmt.Println("Error Parsing Version File " + err.Error())
						os.Exit(1)
					}

					if dbMaxVersionNumber < testVer {
						dbMaxVersionNumber = testVer
					}
				}
			}
		}

		verifyVersion = dbMaxVersionNumber
	} else {
		verifyVersion, err = strconv.Atoi(cfg.verifyValue)
		if err != nil {
			fmt.Println("Error Parsing Verify Version")
			os.Exit(1)
		}
	}

	fmt.Println("Verifying Version ", verifyVersion)

	verFile, err := os.Open(dbBackupVersionFolder + "\\" + strconv.Itoa(verifyVersion))
	if err != nil {
		fmt.Println("Error Opening Version File " + strconv.Itoa(verifyVersion) + " " + err.Error())
		os.Exit(1)
	}

	var verFileHash = ""
	var bVerifyErrors = false
	scanner := bufio.NewScanner(verFile)
	for scanner.Scan() {
		verFileHash = scanner.Text()
		if strings.HasPrefix(verFileHash, "HASH:") {
			newFileHash, err := hashGzipFile(dbBackupFilesFolder + "\\" + verFileHash[5:7] + "\\" + verFileHash[5:])
			if err != nil {
				fmt.Println("Error Hashing File " + dbBackupFilesFolder + "\\" + verFileHash[5:7] + "\\" + verFileHash[5:] + " : " + err.Error())
				bVerifyErrors = true
			} else {
				newStringFileHash := HashToString(newFileHash)

				if newStringFileHash != verFileHash[5:] {
					fmt.Println("File Not Verifyed " + newStringFileHash + "!=" + verFileHash[5:])
					bVerifyErrors = true
				}
			}
		}
	}

	verFile.Close()

	if bVerifyErrors == true {
		os.Exit(1)
	}

	return
}

func FixFiles(cfg Config) {

	exists, err := FolderExists(dbBackupVersionFolder)
	if exists == false || err != nil {
		if err != nil {
			fmt.Println("No Version Folder Found " + err.Error())
			os.Exit(1)
		} else {
			fmt.Println("No Version Folder Found")
			os.Exit(1)
		}
	}

	exists, err = FolderExists(dbBackupFilesFolder)
	if exists == false || err != nil {
		if err != nil {
			fmt.Println("No Files Folder Found " + err.Error())
			os.Exit(1)
		} else {
			fmt.Println("No Files Folder Found")
			os.Exit(1)
		}
	}

	//find max version number
	var dbMaxVersionNumber = 0
	verDirFile, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		fmt.Println("Error Reading Version Files " + err.Error())
		os.Exit(1)
	}
	for _, verDF := range verDirFile {
		if verDF.IsDir() == false {
			if strings.HasSuffix(verDF.Name(), ".tmp") {
				err = FileDelete(dbBackupVersionFolder + "\\" + verDF.Name())
				if err != nil {
					fmt.Println("Error Cleaning Up Temp Version " + verDF.Name() + " " + err.Error())
					os.Exit(1)
				}
			} else {
				testVer, err := strconv.Atoi(verDF.Name())
				if err != nil {
					fmt.Println("Error Parsing Version File " + err.Error())
					os.Exit(1)
				}

				if dbMaxVersionNumber < testVer {
					dbMaxVersionNumber = testVer
				}
			}
		}
	}

	//open version file for reading hashes
	verFiles, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		fmt.Println("Error Reading Version Folder " + err.Error())
		os.Exit(1)
	}

	var toKeep = map[string]bool{}
	for _, verDF := range verFiles {
		fmt.Println("Loading Versin File " + verDF.Name())
		verFile, err := os.Open(dbBackupVersionFolder + "\\" + verDF.Name())
		if err != nil {
			fmt.Println("Error Opening Version File " + verDF.Name() + " " + err.Error())
			os.Exit(1)
		}

		var verFileHash = ""
		scanner := bufio.NewScanner(verFile)
		for scanner.Scan() {
			verFileHash = scanner.Text()
			if strings.HasPrefix(verFileHash, "HASH:") {
				fmt.Println("Adding File Hash " + verFileHash[5:])
				toKeep[verFileHash[5:]] = true
			}
		}
		verFile.Close()
	}

	err = _FixFilesDir(dbBackupFilesFolder, toKeep)
	if err != nil {
		fmt.Println("Error Fixing Files " + err.Error())
		os.Exit(1)
	}

	return
}

func _FixFilesDir(dir string, toKeep map[string]bool) error {

	dirFiles, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, df := range dirFiles {
		if df.IsDir() {
			err := _FixFilesDir(dir+"\\"+df.Name(), toKeep)
			if err != nil {
				return err
			}
		} else {
			fmt.Println("Checking File " + df.Name())
			if toKeep[df.Name()] == false {
				fmt.Println("Deleteing File " + df.Name())
				err = FileDelete(dir + "\\" + df.Name())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func FixFileInUse(cfg Config) {
	//remove the inuse file
	err := FileDelete(dbBackupInUseFile)
	if err != nil {
		fmt.Println("Error Removing In Use File " + err.Error())
		os.Exit(1)
	}
	return
}

func ReadConfig(path string) (Config, error) {
	exists, err := FileExists(path)
	if err != nil || exists == false {
		return Config{}, errors.New("File Does Not Exist")
	}

	data, err := ReadByteSliceOfFile(path)
	if err != nil {
		return Config{}, errors.New("Reading File Error " + err.Error())
	}

	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return Config{}, errors.New("Unmarshaling Error " + err.Error())
	}

	return cfg, nil
}

func WriteConfig(path string, cfg Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return errors.New("Marshaling Error " + err.Error())
	}

	err = WriteByteSliceToFile(path, data)
	if err != nil {
		return errors.New("Writing File Error " + err.Error())
	}

	return nil
}

func hashGzipFile(path string) ([]byte, error) {
	hasher := sha1.New()

	file, err := os.Open(path)
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return []byte{}, err
	}
	defer gz.Close()

	reader := bufio.NewReader(gz)
	_, err = io.Copy(hasher, reader)
	if err != nil {
		return []byte{}, err
	}

	return hasher.Sum(nil), nil
}

func appendHash(b, a []byte) []byte {
	hasher := sha1.New()

	lena := len(a)
	c := make([]byte, lena+len(b))
	for i, v := range a {
		c[i] = v
	}
	for i, v := range b {
		c[lena+i] = v
	}

	hasher.Write(c)
	return hasher.Sum(nil)
}

func appendStringSlice(a, b []string) []string {
	alen := len(a)
	c := make([]string, alen+len(b))
	for i, s := range a {
		c[i] = s
	}
	for i, s := range b {
		c[alen+i] = s
	}
	return c
}

func WriteByteSliceToFile(path string, data []byte) error {
	err := ioutil.WriteFile(path, data, 0644)
	return err
}

func ReadByteSliceOfFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	} else {
		return b, nil
	}
}

func FileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		if f.IsDir() == true {
			return true, errors.New("This Is A Dir")
		} else {
			return true, nil
		}
	} else {
		if os.IsNotExist(err) {
			return false, nil
		}
	}

	return true, err
}

func FolderExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		if f.IsDir() == false {
			return true, errors.New("This Is A File")
		} else {
			return true, nil
		}
	} else {
		if os.IsNotExist(err) {
			return false, nil
		}
	}

	return true, err
}

func FileDelete(path string) error {
	f, err := os.Stat(path)
	if err != nil {
		return err
	} else {
		if f.IsDir() == true {
			return errors.New("Path Is Dir")
		} else {
			err := os.Remove(path)
			return err
		}
	}
}

func MakeDir(path string) error {
	b, err := FolderExists(path)
	if err != nil {
		return err
	} else if b == true {
		return errors.New("Path Exists")
	} else {
		err := os.Mkdir(path, 0644)
		return err
	}
}

// Backup performs a backup operation using the provided configuration
func Backup(cfg Config) error {
	// Validate config
	if cfg.BackupDir == "" {
		return errors.New("backup directory is required")
	}
	if len(cfg.Include) == 0 {
		return errors.New("at least one include path is required")
	}

	// Check if any include paths exist
	validPath := false
	for _, path := range cfg.Include {
		if _, err := os.Stat(path); err == nil {
			validPath = true
			break
		}
	}
	if !validPath {
		return errors.New("no valid include paths found")
	}

	dbBackupFolder = strings.TrimRight(cfg.BackupDir, "\\")
	dbBackupVersionFolder = filepath.Join(dbBackupFolder, "Version")
	dbBackupFilesFolder = filepath.Join(dbBackupFolder, "Files")
	dbBackupInUseFile = filepath.Join(dbBackupFolder, "InUse.txt")

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(dbBackupFolder, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Create version directory if it doesn't exist
	if err := os.MkdirAll(dbBackupVersionFolder, 0755); err != nil {
		return fmt.Errorf("failed to create version directory: %v", err)
	}

	// Create files directory if it doesn't exist
	if err := os.MkdirAll(dbBackupFilesFolder, 0755); err != nil {
		return fmt.Errorf("failed to create files directory: %v", err)
	}

	// Create subdirectories in files directory
	for i := 0; i <= 25; i++ {
		subdir := filepath.Join(dbBackupFilesFolder, fmt.Sprintf("%02d", i))
		if err := os.MkdirAll(subdir, 0755); err != nil {
			return fmt.Errorf("failed to create subfiles directory %s: %v", subdir, err)
		}
	}

	// Check if backup dir is in use
	exists, err := FileExists(dbBackupInUseFile)
	if exists || err != nil {
		if err != nil {
			return fmt.Errorf("error checking in-use file: %v", err)
		}
		return errors.New("backup directory is in use")
	}

	// Mark backup folder in use
	if err := WriteByteSliceToFile(dbBackupInUseFile, []byte{}); err != nil {
		return fmt.Errorf("failed to create in-use file: %v", err)
	}
	defer FileDelete(dbBackupInUseFile)

	BackupFiles(cfg)
	return nil
}

// Trim performs a trim operation using the provided configuration and trim value
func Trim(cfg Config, trimValue string) error {
	cfg.trimValue = trimValue

	// Validate trim value
	_, err := strconv.Atoi(trimValue)
	if err != nil {
		return fmt.Errorf("invalid trim version: %v", err)
	}

	return nil
}

// Verify performs a verify operation using the provided configuration and verify value
func Verify(cfg Config, verifyValue string) error {
	cfg.verifyValue = verifyValue

	// Validate verify value
	if verifyValue != "0" {
		_, err := strconv.Atoi(verifyValue)
		if err != nil {
			return fmt.Errorf("invalid verify version: %v", err)
		}
	}

	return nil
}

// GetFileSize returns the size of a file in MB
func GetFileSize(path string) float64 {
	f, err := os.Stat(path)
	if err != nil {
		return 0
	}
	sizeMB := float64(f.Size()) / 1024.0 / 1024.0
	return sizeMB
}

// GetFileModifiedDate returns the modification time of a file
func GetFileModifiedDate(path string) time.Time {
	f, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return f.ModTime()
}

// HashFile computes the hash of a file
func HashFile(path string) ([]byte, error) {
	hasher := sha1.New()

	file, err := os.Open(path)
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	_, err = io.Copy(hasher, reader)
	if err != nil {
		return []byte{}, err
	}

	return hasher.Sum(nil), nil
}

// HashToString converts a hash to string
func HashToString(hash []byte) string {
	name := ""
	for _, v := range hash {
		name += fmt.Sprintf("%03d", v)
	}
	return name
}

// CopyFileAndGZip copies and compresses a file
func CopyFileAndGZip(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(out)
	defer func() {
		cerr := gzipWriter.Close()
		if err == nil {
			err = cerr
		}
		cerr = out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(gzipWriter, in); err != nil {
		return err
	}
	err = out.Sync()
	if err != nil {
		return err
	}
	return nil
}

// Fix performs a fix operation using the provided configuration
func Fix(cfg Config) error {
	FixFiles(cfg)
	return nil
}

// FixInUse performs a fix in-use operation using the provided configuration
func FixInUse(cfg Config) error {
	FixFileInUse(cfg)
	return nil
}
