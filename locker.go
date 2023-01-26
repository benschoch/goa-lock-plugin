package goa

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"goa.design/goa/v3/codegen"
)

const (
	lockFilePermission     = 0777
	lockFileDirPermissions = 0755
	lockFilename           = "gen/goa.lock"
	outputFlagName         = "output"
)

var (
	errNoFilesDefined         = errors.New("no files defined")
	errFailedToCreateChecksum = errors.New("failed to create checksum from file")
	errFailedToWriteChecksum  = errors.New("failed to write checksum to file")
)

type Locker struct {
	files        []*codegen.File
	lockFilename string
	lockFile     *os.File
}

func NewLocker(files []*codegen.File) (*Locker, error) {
	// read output flag from `goa gen` command
	outputDirFlag := flag.Lookup(outputFlagName)
	outputDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if outputDirFlag != nil && outputDirFlag.Value.String() != "" {
		outputDir = outputDirFlag.Value.String()
	}

	absolutePath, err := filepath.Abs(outputDir + "/" + lockFilename)
	if err != nil {
		return nil, err
	}

	return &Locker{files: files, lockFilename: absolutePath}, nil
}

func (l *Locker) Lock() (lockFilename []byte, err error) {
	if len(l.files) == 0 {
		return nil, errNoFilesDefined
	}

	err = l.prepareLockfile()
	if err != nil {
		return nil, err
	}

	for _, file := range l.files {
		f := file // file must be copied within for loop
		file.FinalizeFunc = func(s string) error {
			return l.postProcessGeneratedFile(s, f)
		}
	}

	return []byte(l.lockFilename), nil
}

func (l *Locker) postProcessGeneratedFile(filename string, file *codegen.File) error {
	checksum, err := l.createChecksum(filename)
	if err != nil {
		return fmt.Errorf("%w (%q): %v", errFailedToCreateChecksum, file.Path, err)
	}
	err = l.writeChecksumToLockFile(file.Path, string(checksum))
	if err != nil {
		return fmt.Errorf("%w (%q): %v", errFailedToWriteChecksum, l.lockFilename, err)
	}

	return nil
}

func (l *Locker) prepareLockfile() error {
	if _, err := os.Stat(filepath.Dir(l.lockFilename)); os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(l.lockFilename), lockFileDirPermissions)
		if err != nil {
			return err
		}
	}

	// create or truncate lock file
	_, err := os.Create(l.lockFilename)

	return err
}

func (l *Locker) writeChecksumToLockFile(file, checksum string) error {
	lockFile, err := os.OpenFile(l.lockFilename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, lockFilePermission)
	if err != nil {
		return err
	}
	defer func(lockFile *os.File) {
		if err := lockFile.Close(); err != nil {
			log.Fatalf("failed to close file lock: %v", err)
		}
	}(lockFile)

	if _, err := lockFile.Write([]byte(fmt.Sprintf("%s::%s\n", file, checksum))); err != nil {
		return err
	}

	return nil
}

func (l *Locker) createChecksum(generatedFilePath string) ([]byte, error) {
	file, err := os.Open(generatedFilePath)
	if err != nil {
		return nil, err
	}
	defer func(fileHandle *os.File) {
		if err := fileHandle.Close(); err != nil {
			log.Fatalf("failed to close file lock: %v", err)
		}
	}(file)

	checksum := sha256.New()
	_, err = io.Copy(checksum, file)
	if err != nil {
		return nil, err
	}

	return []byte(hex.EncodeToString(checksum.Sum(nil))), nil
}
