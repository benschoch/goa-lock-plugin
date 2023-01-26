package goa

import (
	"log"

	"goa.design/goa/v3/codegen"
	"goa.design/goa/v3/eval"
)

// Register the plugin Generator functions.
func init() {
	codegen.RegisterPluginLast("lock", "gen", nil, Generate)
}

// Generate applies finalization functions to all files in order to put their checksums into the lock file
func Generate(_ string, _ []eval.Root, files []*codegen.File) ([]*codegen.File, error) {
	locker, err := NewLocker(files)
	if err != nil {
		return files, err
	}

	lockFile, err := locker.Lock()
	log.Printf("created lock file %q", lockFile)

	return files, err
}
