package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/bvandorf/gitstylebackup/pkg/gitstylebackup"
)

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

func usage() {
	fmt.Printf("%s\n", usageStr)
	os.Exit(-1)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

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
		var eConfig = gitstylebackup.Config{
			BackupDir: "C:\\Temp",
			Include:   []string{"C:\\Users", "C:\\ProgramData"},
			Exclude:   []string{"C:\\Users\\Default"},
		}

		if err := gitstylebackup.WriteConfig(exampleConfig, eConfig); err != nil {
			fmt.Println("Error Writing Example Config File: " + err.Error())
			os.Exit(1)
		}

		os.Exit(0)
	}

	cfg, err := gitstylebackup.ReadConfig(configFilePath)
	if err != nil {
		fmt.Println("Error Reading Config File: " + err.Error())
		os.Exit(1)
	}

	if runBackup {
		if err := gitstylebackup.Backup(cfg); err != nil {
			fmt.Printf("Error during backup: %v\n", err)
			os.Exit(1)
		}
	}

	if runTrim {
		if err := gitstylebackup.Trim(cfg, trimVersionArg); err != nil {
			fmt.Printf("Error during trim: %v\n", err)
			os.Exit(1)
		}
	}

	if runFix {
		if err := gitstylebackup.Fix(cfg); err != nil {
			fmt.Printf("Error during fix: %v\n", err)
			os.Exit(1)
		}
	}

	if runFixInuse {
		if err := gitstylebackup.FixInUse(cfg); err != nil {
			fmt.Printf("Error during fix in-use: %v\n", err)
			os.Exit(1)
		}
	}

	if runVerify {
		if err := gitstylebackup.Verify(cfg, verifyVersionArg); err != nil {
			fmt.Printf("Error during verify: %v\n", err)
			os.Exit(1)
		}
	}
}
