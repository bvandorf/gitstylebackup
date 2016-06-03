package main

import (
	"fmt"
	"flag"
	"os"
	"encoding/json"
	"log"
	"errors"
	"io/ioutil"
	"io"
	"strings"
	"crypto/sha1"
	"bufio"
)

var usageStr = `
Backup Options:
-b, --backup				Use to backup using config file
-t, --trim <date>			Use to trim backup directory to date specified
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
	flag.BoolVar(&runTrim, "t", false, "")
	flag.BoolVar(&runTrim, "trim", false, "")
	
	var runFix bool
	flag.BoolVar(&runFix, "fix", false, "")
	
	flag.Usage = usage
	flag.Parse()
	
	if showHelp {
		usage()
	}
	
	if showVersion {
		fmt.Println("Version 1.0")
		os.Exit(-1)
	}
	
	if (runBackup && runTrim) {
		fmt.Println("You Cant Use both Trim And Backup At The Same Time")
		usage()
	}
	
	if ((runBackup || runTrim) && runFix) {
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
	Include []string
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

func writeConfig(path string, cfg Config) (error) {
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
	Inuse bool
	Versions []bdb_version
}

type bdb_version struct {
	Number int
	Dir []bdb_version_dir
}

type bdb_version_dir struct {
	Path string
	Files []bdb_version_dir_file
}

type bdb_version_dir_file struct {
	Name string
	Hash []byte
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

func writeDB(path string, db bdb) (error) {
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

type tempFile struct {
	path map[string]tempFilePros
}

type tempFilePros struct {
	hash []byte
	dirty bool
	deleted bool
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

func testEq(a, b []byte) (bool) {
	if a == nil && b == nil {
		return true
	}
	
	if a == nil || b == nil {
		return false
	}
	
	if len(a) != len(b) {
		return false
	}
	
	for i:= range a {
		if a[i] != b[i] {
			return false
		}
	}
	
	return true
}

func BackupFiles(cfg Config) (error) {
	var db = bdb{}
	
	var dbFilePath = strings.TrimRight(cfg.BackupDir, "\\")
	dbFilePath += "\\backup.db"
	
	//look for backup db
	exists, err := FileExists(dbFilePath)
	if exists == true && err == nil {
		tmpdb, err := readDB(dbFilePath)
		if err != nil {
			return errors.New("Reading DB Error " + err.Error())
		}
		db = tmpdb
	} else if (exists == true && err != nil) {
		return errors.New("Reading DB Error " + err.Error())
	}
	
	//add inuse fal to file
	db.Inuse = true;
	
	err = writeDB(dbFilePath, db)
	if err != nil {
		return err
	}
	
	//find oldest version number
	var dbVersionNumber = 0
	var dbVersionIndex = -1
	for i,v := range db.Versions {
		if dbVersionNumber < v.Number {
			dbVersionNumber = v.Number
			dbVersionIndex = i
		}
	}
	
	var tempFileList = tempFile{}
	tempFileList.path = map[string]tempFilePros{}
	var tempFileListProp = tempFilePros{}
	
	if dbVersionIndex >= 0 {
		//read all files and hashes to temp list for oldest version number
		for _,d := range db.Versions[dbVersionNumber].Dir {
			for _,f := range d.Files {
				tempFileListProp.deleted = true
				tempFileListProp.dirty = false
				tempFileListProp.hash = f.Hash
				tempFileList.path[d.Path + "/" + f.Name] = tempFileListProp
			}
		}
	}
	
	//run thru each dir/file in include config
	for _,cd := range cfg.Include {
		//check if dir in config is valid
		exists, err := FolderExists(cd)
		if (exists == true && err == nil) {
			//backup folder
			files, err := buildListOfFiles(cd)
			if err == nil {
				for _,f := range files {
					//file in folder
					tempFileListProp = tempFileList.path[f]
					
					tempFileListProp.deleted = false
					newHash, err := hashFile(f)
					if err == nil {
						if !testEq(tempFileListProp.hash, newHash) {
							tempFileListProp.dirty = true
							tempFileListProp.hash = newHash
						}
					} else {
						fmt.Println("Error Hashing " + f)
					}
					tempFileList.path[f] = tempFileListProp
				}
			} else {
				fmt.Println("Error Getting Files From " + cd)				
			}
		} else if (exists == true && err != nil) {
			//backup file
			tempFileListProp = tempFileList.path[cd]
			
			tempFileListProp.deleted = false
			newHash, err := hashFile(cd)
			if err == nil {
				if !testEq(tempFileListProp.hash, newHash) {
					tempFileListProp.dirty = true
					tempFileListProp.hash = newHash
				}
			} else {
				fmt.Println("Error Hashing " + cd)
			}
		}
	}
	
	for key,val := range tempFileList.path {
		fmt.Println(key + " -> ", val.hash)
	}
	
	//copy new and updateed file to dest dir
	
	//update db and remove inuse flag
	
	return nil	
}

func TrimFiles(cfg Config) (error) {
	return nil	
}

func FixFiles(cfg Config) (error) {
	return nil	
}

func buildListOfFiles(dir string) ([]string, error) {
	files := []string{}
	dirFiles, err := ioutil.ReadDir(dir)
	if err != nil {
		return []string{}, err
	}

	for _,df := range dirFiles {
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

func appendStringSlice(a, b []string) ([]string) {
	alen := len(a)
	c := make([]string, alen + len(b))
	for i,s := range a {
		c[i] = s
	}
	for i,s := range b {
		c[alen + i] = s
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