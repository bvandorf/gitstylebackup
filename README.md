# gitstylebackup
Git Style File Backup Program

Creates a file deduplicated system backup based on config file and only saves new or changed files based on a sha1 of the file contents in the backup directory structure shown below.

Each version will contain the same reference to if the file has not changed or there are duplicate files that were backed up.

# gitstylebackupexplorer

The application to view/restore files and directories from your backup <br>
https://github.com/bvandorf/gitstylebackupexplorer

# Backup Directory Structure
```
/RootFolder  -  Main backup folder for all operations
  /Versions  -  Version folder that holds each of the backup version information
  /Files     -  Files folder that holds folders starting with the hash of the file
    /00      -  Hash folder containing the files that hash starts wth 00
    ...
```
# Config File
```
{
    "BackupDir": "c:\\backups\\gitstylebackup",
    "Include": [
        "C:\\temp",
    "C:\\Users"
    ],
    "Exclude": [
        "C:\\Users\\Default\\",
    "C:\\temp\\exclude file.txt"
    ]
}
```
# Command Line Options
```
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
```

# Usage Examples
```
gitstylebackup -b 

When useing ShadowSpawn config file must reference shado copy drive letter
ShadowSpawn.exe c:\ z: c:\backup\gitstylebackup.exe -b -c c:\backup\config.txt
```

# Shadow Spawn
https://github.com/candera/shadowspawn