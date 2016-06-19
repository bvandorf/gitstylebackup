package main

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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU() - 1)
}

var usageStr = `
Backup Options:
-b, --backup                Use to backup using config file
-t, --trim <version>        Use to trim backup directory to version's specified
           <+x>             Use to trim backup directory to keep current + x version's specified
-c, --config <file>         Use to specify the config file used (default: config.txt)
    --exampleconfig <file>  Use to make an example config file
    --fix                   Use to fix interrupted backup or trim
    --fixinuse              Use to remove inuse flag from backup

Common Options:
-h, --help                  Show this help
-v, --version               Show version

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

type Config struct {
	BackupDir string
	Include   []string
	Exclude   []string
	trimValue string `json:"-"`
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
	flag.BoolVar(&showVersion, "v", false, "")
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

	flag.Usage = usage
	flag.Parse()

	if trimVersionArg != "" {
		runTrim = true
	}

	if showHelp {
		usage()
	}

	if showVersion {
		fmt.Println("Version 1.0")
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

		if err := writeConfig(exampleConfig, eConfig); err != nil {
			fmt.Println("Error Writing Example Config File: " + err.Error())
			os.Exit(1)
		}

		os.Exit(0)
	}

	cfg, err := readConfig(configFilePath)
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
		err := BackupFiles(cfg)
		if err != nil {
			fmt.Println("Error Backing Up Files: " + err.Error())
			os.Exit(1)
		}
	}

	if runTrim {
		cfg.trimValue = trimVersionArg
		err := TrimFiles(cfg)
		if err != nil {
			fmt.Println("Error Trimming Backup Files: " + err.Error())
			os.Exit(1)
		}
	}

	if runFix {
		err := FixFiles(cfg)
		if err != nil {
			fmt.Println("Error Fixing Backup Files: " + err.Error())
			os.Exit(1)
		}
	}

	if runFixInuse {
		err := FixFileInUse(cfg)
		if err != nil {
			fmt.Println("Error Fixing Backup Files: " + err.Error())
			os.Exit(1)
		}
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
			return errors.New("Makeing Version Folder " + err.Error())
		}
	}

	exists, err = FolderExists(dbBackupFilesFolder)
	if exists == false && err == nil {
		err = MakeDir(dbBackupFilesFolder)
		if err != nil {
			return errors.New("Makeing Files Folder " + err.Error())
		}

		for i := 0; i <= 25; i++ {
			err = MakeDir(dbBackupFilesFolder + "\\" + fmt.Sprintf("%02d", i))
			if err != nil {
				return errors.New("Makeing SubFiles Folder " + err.Error())
			}
		}
	}

	//find max version number
	var dbNewVersionNumber = 0
	verDirFile, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		return errors.New("Reading Version Files " + err.Error())
	}
	for _, verDF := range verDirFile {
		if verDF.IsDir() == false {
			if strings.HasSuffix(verDF.Name(), ".tmp") {
				err = FileDelete(dbBackupVersionFolder + "\\" + verDF.Name())
				if err != nil {
					return errors.New("Cleaning Up Temp Version " + verDF.Name() + " " + err.Error())
				}
			} else {
				testVer, err := strconv.Atoi(verDF.Name())
				if err != nil {
					return errors.New("Parsing Version File " + err.Error())
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
		errors.New("Opening Version File " + err.Error())
	}

	_, err = verFile.WriteString("VERSION:" + strconv.Itoa(dbNewVersionNumber) + fileNewLine)
	if err != nil {
		return errors.New("Writeing Version File " + err.Error())
	}

	_, err = verFile.WriteString("DATE:" + time.Now().Format(timeFormat) + fileNewLine)
	if err != nil {
		return errors.New("Writeing Version File " + err.Error())
	}

	var verHash []byte
	for _, cd := range cfg.Include {
		//check if dir in config is valid
		exists, err := FolderExists(cd)
		if exists == true && err == nil {
			//backup folder
			tempHash, err := _BackupFilesFolder(cd, cfg.Exclude, dbBackupFilesFolder, verFile)
			if err != nil {
				return err
			}
			verHash = appendHash(verHash, tempHash)
		} else if exists == true && err != nil {
			//backup file
			tempHash, err := _BackupFilesFolder(cd, cfg.Exclude, dbBackupFilesFolder, verFile)
			if err != nil {
				return err
			}
			verHash = appendHash(verHash, tempHash)
		} else {
			fmt.Println("Error Backing Up " + cd + " No Fle Or Folder Exists")
		}
	}

	_, err = verFile.WriteString("VERSIONHASH:" + hashToString(verHash) + fileNewLine)
	if err != nil {
		return errors.New("Writeing Version File " + err.Error())
	}

	wg.Wait()

	verFile.Close()

	err = os.Rename(dbBackupNewTempVersionFile, dbBackupNewVersionFile)
	if err != nil {
		return errors.New("Renameing Version File " + err.Error())
	}

	return nil
}

var wg sync.WaitGroup

func _BackupFilesFolder(path string, exclude []string, dbFilesPath string, verFile *os.File) ([]byte, error) {
	for _, dir := range exclude {
		dir = strings.TrimRight(dir, "\\")
		if strings.HasPrefix(path, dir) {
			return []byte{}, nil
		}
	}

	var FolderHash []byte
	dirFiles, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println("Error Reading Folder " + path + " " + err.Error())
		return []byte{}, nil
	}

	for _, df := range dirFiles {
		if df.IsDir() {
			tempHash, err := _BackupFilesFolder(path+"\\"+df.Name(), exclude, dbFilesPath, verFile)
			if err != nil {
				return []byte{}, nil
			}
			FolderHash = appendHash(FolderHash, tempHash)
		} else {
			var bExclude = false
			for _, dir := range exclude {
				dir = strings.TrimRight(dir, "\\")
				if strings.HasPrefix(path+"\\"+df.Name(), dir) {
					bExclude = true
				}
			}

			if bExclude == false {
				fileHash, err := hashFile(path + "\\" + df.Name())
				if err != nil {
					fmt.Println("Error Hashing File " + path + "\\" + df.Name() + " " + err.Error())
				}

				sFileHash := hashToString(fileHash)

				_, err = verFile.WriteString("FILE:" + path + "\\" + df.Name() + fileNewLine)
				if err != nil {
					return []byte{}, errors.New("Writeing Version File " + err.Error())
				}

				_, err = verFile.WriteString("MODDATE:" + getFileModifiedDate(path+"\\"+df.Name()).Format(timeFormat) + fileNewLine)
				if err != nil {
					return []byte{}, errors.New("Writeing Version File " + err.Error())
				}

				_, err = verFile.WriteString("SIZE:" + strconv.FormatFloat(getFileSize(path+"\\"+df.Name()), 'f', 6, 64) + fileNewLine)
				if err != nil {
					return []byte{}, errors.New("Writeing Version File " + err.Error())
				}

				_, err = verFile.WriteString("HASH:" + sFileHash + fileNewLine)
				if err != nil {
					return []byte{}, errors.New("Writeing Version File " + err.Error())
				}

				exists, err := FileExists(dbFilesPath + "\\" + sFileHash[:2] + "\\" + sFileHash)
				if exists == false && err == nil {
					fmt.Println("COPYING FILE:" + path + "\\" + df.Name() + " -> " + sFileHash)
					wg.Add(1)
					go func(gosrcpath, godstpath string, gowg *sync.WaitGroup) {
						defer wg.Done()
						goerr := CopyFileAndGZip(gosrcpath, godstpath)
						if goerr != nil {
							fmt.Println("Error Copying File " + gosrcpath + " " + goerr.Error())
						}
					}(path+"\\"+df.Name(), dbFilesPath+"\\"+sFileHash[:2]+"\\"+sFileHash, &wg)
				} else if exists && err == nil {
					fmt.Println("SKIP FILE COPY:" + path + "\\" + df.Name() + " -> " + sFileHash)
				} else {
					fmt.Println("Error Copying File: " + path + "\\" + df.Name() + " " + err.Error())
				}

				FolderHash = appendHash(FolderHash, fileHash)
			}
		}
	}

	return FolderHash, nil
}

func TrimFiles(cfg Config) error {

	exists, err := FolderExists(dbBackupVersionFolder)
	if exists == false || err != nil {
		if err != nil {
			return errors.New("No Version Folder Found " + err.Error())
		} else {
			return errors.New("No Version Folder Found ")
		}
	}

	exists, err = FolderExists(dbBackupFilesFolder)
	if exists == false || err != nil {
		if err != nil {
			return errors.New("No Files Folder Found " + err.Error())
		} else {
			return errors.New("No Files Folder Found ")
		}
	}

	//find max version number
	var dbMaxVersionNumber = 0
	verDirFile, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		return errors.New("Reading Version Files " + err.Error())
	}
	for _, verDF := range verDirFile {
		if verDF.IsDir() == false {
			if strings.HasSuffix(verDF.Name(), ".tmp") {
				err = FileDelete(dbBackupVersionFolder + "\\" + verDF.Name())
				if err != nil {
					return errors.New("Cleaning Up Temp Version " + verDF.Name() + " " + err.Error())
				}
			} else {
				testVer, err := strconv.Atoi(verDF.Name())
				if err != nil {
					return errors.New("Parsing Version File " + err.Error())
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
		return err
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
		errors.New("Reading Version Folder " + err.Error())
	}

	var toDel = map[string]bool{}
	for _, verDF := range verFiles {
		fmt.Println("Loading Version File " + verDF.Name())
		testVer, err := strconv.Atoi(verDF.Name())
		if err != nil {
			return errors.New("Parsing Version File " + err.Error())
		}
		if testVer < trimVersion {
			verFile, err := os.Open(dbBackupVersionFolder + "\\" + verDF.Name())
			if err != nil {
				return errors.New("Opening Version File " + verDF.Name() + " " + err.Error())
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
			return errors.New("Parsing Version File " + err.Error())
		}
		if testVer >= trimVersion {
			verFile, err := os.Open(dbBackupVersionFolder + "\\" + verDF.Name())
			if err != nil {
				return errors.New("Opening Version File " + verDF.Name() + " " + err.Error())
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
				return errors.New("Deleteing Versin File " + strconv.Itoa(ver) + " " + err.Error())
			}
		} else if err != nil {
			return errors.New("Deleteing Version File " + strconv.Itoa(ver) + " " + err.Error())
		}
	}

	return nil
}

func FixFiles(cfg Config) error {

	exists, err := FolderExists(dbBackupVersionFolder)
	if exists == false || err != nil {
		if err != nil {
			return errors.New("No Version Folder Found " + err.Error())
		} else {
			return errors.New("No Version Folder Found ")
		}
	}

	exists, err = FolderExists(dbBackupFilesFolder)
	if exists == false || err != nil {
		if err != nil {
			return errors.New("No Files Folder Found " + err.Error())
		} else {
			return errors.New("No Files Folder Found ")
		}
	}

	//find max version number
	var dbMaxVersionNumber = 0
	verDirFile, err := ioutil.ReadDir(dbBackupVersionFolder)
	if err != nil {
		return errors.New("Reading Version Files " + err.Error())
	}
	for _, verDF := range verDirFile {
		if verDF.IsDir() == false {
			if strings.HasSuffix(verDF.Name(), ".tmp") {
				err = FileDelete(dbBackupVersionFolder + "\\" + verDF.Name())
				if err != nil {
					return errors.New("Cleaning Up Temp Version " + verDF.Name() + " " + err.Error())
				}
			} else {
				testVer, err := strconv.Atoi(verDF.Name())
				if err != nil {
					return errors.New("Parsing Version File " + err.Error())
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
		return errors.New("Reading Version Folder " + err.Error())
	}

	var toKeep = map[string]bool{}
	for _, verDF := range verFiles {
		fmt.Println("Loading Versin File " + verDF.Name())
		verFile, err := os.Open(dbBackupVersionFolder + "\\" + verDF.Name())
		if err != nil {
			return errors.New("Opening Version File " + verDF.Name() + " " + err.Error())
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
		return errors.New("Fixing Files " + err.Error())
	}

	return nil
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

func FixFileInUse(cfg Config) error {

	//remove the inuse file
	err := FileDelete(dbBackupInUseFile)
	if err != err {
		return errors.New("Error Removeing In Use File " + err.Error())
	}

	return nil
}

func readConfig(path string) (Config, error) {
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

func writeConfig(path string, cfg Config) error {
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

var hasher = sha1.New()

func hashFile(path string) ([]byte, error) {
	hasher.Reset()

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

func appendHash(b, a []byte) []byte {
	hasher.Reset()

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

func getFileSize(path string) float64 {
	f, err := os.Stat(path)
	if err != nil {
		return 0
	}
	sizeMB := float64(f.Size()) / 1024.0 / 1024.0
	return sizeMB
}

func getFileModifiedDate(path string) time.Time {
	f, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return f.ModTime()
}

func hashToString(hash []byte) string {
	name := ""
	for _, v := range hash {
		name += fmt.Sprintf("%03d", v)
	}
	return name
}

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
