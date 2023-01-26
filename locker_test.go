package goa

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"goa.design/goa/v3/codegen"
)

var (
	_ = flag.String(outputFlagName, "", "")
)

func TestLocker_NewLocker(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	tests := []struct {
		name             string
		wantLockFilename string
		outputFlagValue  string
	}{
		{
			name:             "default",
			wantLockFilename: wd + "/gen/goa.lock",
		},
		{
			name:             "with output flag",
			wantLockFilename: wd + "/tmp/gen/goa.lock",
			outputFlagValue:  "tmp",
		},
		{
			name:             "normalize path",
			wantLockFilename: "/foo/gen/goa.lock",
			outputFlagValue:  "///foo/bar/..//",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := flag.Set(outputFlagName, test.outputFlagValue)
			assert.NoError(t, err)

			l, err := NewLocker(nil)
			assert.NoError(t, err)
			assert.Equal(t, test.wantLockFilename, l.lockFilename)
		})
	}
}

func TestLocker_prepareLockfile(t *testing.T) {
	tests := []struct {
		name             string
		wantLockFilename string
		outputFlagValue  string
		wantErr          bool
	}{
		{
			name: "default",
		},
		{
			name:            "with output flag",
			outputFlagValue: "tmp",
		},
		{
			name:            "normalize path",
			outputFlagValue: "///foo/bar/..//",
			wantErr:         true, // cannot create directory at root
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := flag.Set(outputFlagName, test.outputFlagValue)
			assert.NoError(t, err)

			l, err := NewLocker(nil)
			assert.NoError(t, err)

			err = l.prepareLockfile()
			if test.wantErr {
				assert.Error(t, err)
			} else {
				// assert lock file has been created
				assert.NoError(t, err)
				_, err = os.Stat(l.lockFilename)
				dirInfo, err := os.Stat(filepath.Dir(l.lockFilename))
				assert.NoError(t, err)
				assert.True(t, dirInfo.IsDir())
			}
		})
	}
}

func TestLocker_Lock(t *testing.T) {
	err := flag.Set(outputFlagName, "")
	assert.NoError(t, err)

	tempFile1 := codegen.CreateTempFile(t, "foo")
	tempFile2 := codegen.CreateTempFile(t, "bar")
	tempFile3 := codegen.CreateTempFile(t, "baz")
	tests := []struct {
		name                  string
		files                 []*codegen.File
		wantLockFilename      []byte
		wantErr               error
		wantLockFileChecksums []string
	}{
		{
			name:    "no files",
			wantErr: errNoFilesDefined,
		},
		{
			name: "single file",
			files: []*codegen.File{
				{
					Path: tempFile1,
					SectionTemplates: []*codegen.SectionTemplate{
						codegen.Header("", "foo", nil),
					},
				},
			},
			wantLockFileChecksums: []string{
				"e47e993bd2881703b09353fffbf73fc88e52e9835a7b47f88f798632e6083d86",
			},
		},
		{
			name: "multiple files",
			files: []*codegen.File{
				{
					Path:             tempFile1,
					SectionTemplates: []*codegen.SectionTemplate{codegen.Header("", "foo", nil)},
				},
				{
					Path:             tempFile2,
					SectionTemplates: []*codegen.SectionTemplate{codegen.Header("", "bar", nil)},
				},
				{
					Path:             tempFile3,
					SectionTemplates: []*codegen.SectionTemplate{codegen.Header("", "baz", nil)},
				},
			},
			wantLockFileChecksums: []string{
				"268daf0fd4a400afc3ac9ce485a68728b41608be29e5e6695a2e878abe609f50",
				"ab7a7803ea891f942c6e2bdd26084eafb1ea4805ed027c5a22cf2719caebc974",
				"4ad475d310ea8f97a8d55893c340fedf3d825debee3ffd649907fda9e8b6630d",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// apply lock
			l, err := NewLocker(test.files)
			assert.NoError(t, err)
			gotLockFilename, err := l.Lock()
			assert.Equal(t, test.wantErr, err)

			// execute goa render function to apply lock per file
			for _, f := range test.files {
				filename, err := f.Render(os.TempDir())
				assert.NotEmpty(t, filename)
				assert.NoError(t, err)
			}
			if test.wantErr != nil {
				return
			}

			// read lock file content
			content, err := os.ReadFile(string(gotLockFilename))
			assert.NoError(t, err)
			assert.NotEmpty(t, content)
			lines := strings.Split(string(content), "\n")

			// assert all checksums exist in lock file
			found := 0
			for _, checksum := range test.wantLockFileChecksums {
				for _, line := range lines {
					hasSuffix := strings.HasSuffix(line, "::"+string(checksum))
					if hasSuffix {
						found++
					}
				}
			}
			expectedNumberOfChecksums := len(test.wantLockFileChecksums)
			expectedNumberOfLines := expectedNumberOfChecksums + 1 // including empty line at end of file
			assert.Equal(t, expectedNumberOfLines, len(lines))
			assert.Equal(t, expectedNumberOfChecksums, found, "failed to find all expected checksums")
		})
	}
}
