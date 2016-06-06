package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var usageStr = `
Backup Options:
-b, --backup				Use to backup using config file
-t, --trim <version>		Use to trim backup directory to version's specified
           <-x>             Use to trim backup directory to keep x version's specified
-c, --config <file>			Use to specify the config file used (default: config.txt)
    --exampleconfig <file>	Use to make an example config file
	--fix					Use to fix interupted backup or trim
	
Common Options:
-h, --help					Show this help
-v, --version				Show version

Exit Codes:
	 0 = Clean
	-1 = Version or help
	 1 = Error
`

func usage() {
	fmt.Printf("%s\n", usageStr)
	os.Exit(-1)
}

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

	if runBackup && runTrim {
		fmt.Println("You Cant Use both Trim And Backup At The Same Time")
		usage()
	}

	if (runBackup || runTrim) && runFix {
		fmt.Println("You Cant Use Trim Or Backup At The Same Time As Fix")
		usage()
	}

	if exampleConfig != "" {
		var eConfig = Config{}
		eConfig.BackupDir = "c:\\temp"
		eConfig.Include = append(eConfig.Include, "c:\\users")
		eConfig.Include = append(eConfig.Include, "c:\\programdata")

		if err := writeConfig(exampleConfig, eConfig); err != nil {
			log.Fatal("Error Writeing Example Config File: " + err.Error())
		}

		os.Exit(0)
	}

	cfg, err := readConfig(configFilePath)
	if err != nil {
		log.Fatal("Error Reading Config File: " + err.Error())
	}

	if runBackup {
		err := BackupFiles(cfg)
		if err != nil {
			log.Fatal("Error Backing Up Files: " + err.Error())
		}
	}

	if runTrim {
		cfg.trimValue = trimVersionArg
		err := TrimFiles(cfg)
		if err != nil {
			log.Fatal("Error Triming Backup Files: " + err.Error())
		}
	}

	if runFix {
		err := FixFiles(cfg)
		if err != nil {
			log.Fatal("Error Fixing Backup Files: " + err.Error())
		}
	}
}

type Config struct {
	BackupDir string
	Include   []string
	trimValue string `json:"-"`
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
		return errors.New("Writeing File Error " + err.Error())
	}

	return nil
}

type bdb struct {
	Inuse   bool
	Version map[string]bdb_version
}

type bdb_version struct {
	Number int
	File   map[string]bdb_version_file
	Hash   []byte
	Date   time.Time
}

type bdb_version_file struct {
	Name    string
	Hash    []byte
	Date    time.Time
	deleted bool `json:"-"`
	dirty   bool `json:"-"`
}

func readDB(path string) (bdb, error) {
	exists, err := FileExists(path)
	if err != nil || exists == false {
		return bdb{}, errors.New("File Does Not Exist")
	}

	data, err := ReadByteSliceOfFile(path)
	if err != nil {
		return bdb{}, errors.New("Reading DB Error " + err.Error())
	}

	var db = bdb{}
	err = json.Unmarshal(data, &db)
	if err != nil {
		return bdb{}, errors.New("Unmarshaling File Error " + err.Error())
	}

	return db, nil
}

func writeDB(path string, db bdb) error {
	data, err := json.Marshal(db)
	if err != nil {
		return errors.New("Marshalling db Error " + err.Error())
	}

	err = WriteByteSliceToFile(path, data)
	if err != nil {
		return errors.New("Writeing db Error " + err.Error())
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

func testEq(a, b []byte) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func StringArrayContains(input []string, has string) bool {
	for _, s := range input {
		if s == has {
			return true
		}
	}
	return false
}

func BackupFiles(cfg Config) error {
	var db = bdb{}

	var dbBackupFolder = strings.TrimRight(cfg.BackupDir, "\\")
	var dbFilePath = dbBackupFolder + "\\backup.db"

	//look for backup db
	exists, err := FileExists(dbFilePath)
	if exists == true && err == nil {
		tmpdb, err := readDB(dbFilePath)
		if err != nil {
			return errors.New("Reading DB Error " + err.Error())
		}
		db = tmpdb
	} else if exists == true && err != nil {
		return errors.New("Reading DB Error " + err.Error())
	}

	//add inuse fal to file
	db.Inuse = true

	err = writeDB(dbFilePath, db)
	if err != nil {
		return err
	}

	//find oldest version number
	var dbVersionNumber = 0
	for _, v := range db.Version {
		if dbVersionNumber < v.Number {
			dbVersionNumber = v.Number
		}
	}

	var sdbVersionNumber = strconv.Itoa(dbVersionNumber)
	var sdbNewVersionNumber = strconv.Itoa(dbVersionNumber + 1)

	var tempDB = bdb_version{}
	tempDB.Number = dbVersionNumber + 1
	tempDB.File = map[string]bdb_version_file{}
	tempDB.Date = time.Now()
	var tempDBFile = bdb_version_file{}

	if dbVersionNumber > 0 {
		//read all files and hashes to temp list for oldest version number
		for _, f := range db.Version[sdbVersionNumber].File {
			tempDBFile.deleted = true
			tempDBFile.dirty = false
			tempDBFile.Name = f.Name
			tempDBFile.Hash = f.Hash
			tempDBFile.Date = f.Date

			tempDB.File[f.Name] = tempDBFile
			tempDB.Hash = appendHash(tempDB.Hash, tempDBFile.Hash)
		}
	} else {
		db.Version = map[string]bdb_version{}
	}

	//run thru each dir/file in include config
	versionHash := []byte{}
	for _, cd := range cfg.Include {
		//check if dir in config is valid
		exists, err := FolderExists(cd)
		if exists == true && err == nil {
			//backup folder
			files, err := buildListOfFiles(cd)
			if err == nil {
				for _, f := range files {
					//file in folder
					tempDBFile = tempDB.File[f]

					tempDBFile.deleted = false
					newHash, err := hashFile(f)
					if err == nil {
						if !testEq(tempDBFile.Hash, newHash) {
							tempDBFile.Name = f
							tempDBFile.dirty = true
							tempDBFile.Hash = newHash
							tempDBFile.Date = time.Now()
						}
						versionHash = appendHash(versionHash, newHash)
						tempDB.File[f] = tempDBFile
					} else {
						fmt.Println("Error Hashing " + f)
					}
					tempDB.File[f] = tempDBFile
				}
			} else {
				fmt.Println("Error Getting Files From " + cd)
			}
		} else if exists == true && err != nil {
			//backup file
			tempDBFile = tempDB.File[cd]

			tempDBFile.deleted = false
			newHash, err := hashFile(cd)
			if err == nil {
				if !testEq(tempDBFile.Hash, newHash) {
					tempDBFile.Name = cd
					tempDBFile.dirty = true
					tempDBFile.Hash = newHash
					tempDBFile.Date = time.Now()
				}
				versionHash = appendHash(versionHash, newHash)
				tempDBFile.Hash = newHash
				tempDB.File[cd] = tempDBFile
			} else {
				fmt.Println("Error Hashing " + cd)
			}
		}
	}
	tempDB.Hash = versionHash

	//copy new and updateed file to dest dir
	for key, val := range tempDB.File {
		if val.dirty {
			exists, _ := FileExists(dbBackupFolder + "\\" + hashToFileName(val.Hash))
			if exists == false {
				fmt.Println("UPDATE FILE: "+key+" -> ", val.Hash)
				err := CopyFile(val.Name, dbBackupFolder+"\\"+hashToFileName(val.Hash))
				if err != nil {
					fmt.Println("Error Copying File " + err.Error())
				}
			} else {
				fmt.Println("SKIPING FILE: "+key+" -> ", val.Hash)
			}
		}
		if val.deleted {
			fmt.Println("SOURCE FILE DELETED: "+key+" -> ", val.Hash)
			delete(tempDB.File, key)
		}
	}

	//update db and remove inuse flag
	newVersion := bdb_version{}
	newVersion.Number = tempDB.Number
	newVersion.File = tempDB.File
	newVersion.Hash = tempDB.Hash

	db.Version[sdbNewVersionNumber] = newVersion

	writeDB(dbFilePath, db)
	if err != nil {
		return err
	}

	return nil
}

func TrimFiles(cfg Config) error {
	var db = bdb{}

	var dbBackupFolder = strings.TrimRight(cfg.BackupDir, "\\")
	var dbFilePath = dbBackupFolder + "\\backup.db"

	//look for backup db
	exists, err := FileExists(dbFilePath)
	if exists == true && err == nil {
		tmpdb, err := readDB(dbFilePath)
		if err != nil {
			return errors.New("Reading DB Error " + err.Error())
		}
		db = tmpdb
	} else if exists == true && err != nil {
		return errors.New("Reading DB Error " + err.Error())
	} else if exists == false {
		return errors.New("No Backup Files To Trim")
	}

	//add inuse fal to file
	db.Inuse = true

	err = writeDB(dbFilePath, db)
	if err != nil {
		return err
	}

	//find the oldest db version
	var dbMaxVersionNumber = 0
	for _, v := range db.Version {
		if dbMaxVersionNumber < v.Number {
			dbMaxVersionNumber = v.Number
		}
	}

	//find what versin to trim to
	trimVersion, err := strconv.Atoi(cfg.trimValue)
	if err != nil {
		return err
	}

	if strings.Contains(cfg.trimValue, "-") {
		trimVersion = dbMaxVersionNumber - trimVersion
	}

	//check if this is valid trim version
	if dbMaxVersionNumber < trimVersion {
		errors.New("Trim To Version " + strconv.Itoa(trimVersion) + " Does Not Exist")
	}

	for ver := 0; ver < trimVersion; ver++ {
		for key, _ := range db.Version[strconv.Itoa(ver)].File {
			err := FileDelete(key)
			if err != nil {
				fmt.Println("Error Deleting File " + key)
			}
		}
		delete(db.Version, strconv.Itoa(ver))
	}

	db.Inuse = false

	writeDB(dbFilePath, db)
	if err != nil {
		return err
	}

	return nil
}

func FixFiles(cfg Config) error {
	var db = bdb{}

	var dbBackupFolder = strings.TrimRight(cfg.BackupDir, "\\")
	var dbFilePath = dbBackupFolder + "\\backup.db"

	//look for backup db
	exists, err := FileExists(dbFilePath)
	if exists == true && err == nil {
		tmpdb, err := readDB(dbFilePath)
		if err != nil {
			return errors.New("Reading DB Error " + err.Error())
		}
		db = tmpdb
	} else if exists == true && err != nil {
		return errors.New("Reading DB Error " + err.Error())
	} else if exists == false {
		return errors.New("No Backup Files To Trim")
	}

	//add inuse fal to file
	db.Inuse = true

	err = writeDB(dbFilePath, db)
	if err != nil {
		return err
	}

	//clean up any files not in the db

	//make a quick list of the files in the db
	var dbfiles = []string{}
	for _, ver := range db.Version {
		for f := range ver.File {
			dbfiles = appendStringSlice(dbfiles, []string{f})
		}
	}

	//make a list of files in the backup folder
	files, err := buildListOfFiles(dbBackupFolder)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f != dbFilePath {
			if StringArrayContains(dbfiles, f) == false {
				FileDelete(f)
			}
		}
	}

	db.Inuse = false

	writeDB(dbFilePath, db)
	if err != nil {
		return err
	}

	return nil
}

func hashToFileName(hash []byte) string {
	name := ""
	for _, v := range hash {
		name += fmt.Sprintf("%03d", v)
	}
	return name
}

func buildListOfFiles(dir string) ([]string, error) {
	files := []string{}
	dirFiles, err := ioutil.ReadDir(dir)
	if err != nil {
		return []string{}, err
	}

	for _, df := range dirFiles {
		if df.IsDir() {
			tmpFiles, err := buildListOfFiles(dir + "\\" + df.Name())
			if err == nil {
				files = appendStringSlice(files, tmpFiles)
			}
		} else {
			files = appendStringSlice(files, []string{dir + "/" + df.Name()})
		}
	}

	return files, nil
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	err = out.Sync()
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
