package rssole

import (
	"log"
	"os"
	"testing"
)

var (
	tempDir string
)

func feedsSetUpTearDown(t *testing.T) func(t *testing.T) {
	// We don't want to make a mess of the local fs
	// so clobber the readcache with one that uses a tmp file.
	var err error
	tempDir, err = os.MkdirTemp("", "Test_Feeds")
	if err != nil {
		log.Fatal(err)
	}

	return func(t *testing.T) {
		os.RemoveAll(tempDir)
	}
}

func TestReadFeedsFile_Success(t *testing.T) {
	defer feedsSetUpTearDown(t)(t)

	file, err := os.CreateTemp(tempDir, "*")
	if err != nil {
		log.Fatal(err)
	}

	// make it valid json
	_, _ = file.WriteString("{}")

	f := feeds{}

	err = f.readFeedsFile(file.Name())

	if err != nil {
		t.Fatal("unexpected error calling readFeedsFile on good", err)
	}
}

func TestReadFeedsFile_BadJson(t *testing.T) {
	defer feedsSetUpTearDown(t)(t)

	file, err := os.CreateTemp(tempDir, "*")
	if err != nil {
		log.Fatal(err)
	}

	_, _ = file.WriteString("NOT_VALID_JSON")

	f := feeds{}

	err = f.readFeedsFile(file.Name())

	if err == nil {
		t.Fatal("expected error calling readFeedsFile on bad json", err)
	}
}

func TestReadFeedsFile_NoSuchFile(t *testing.T) {
	f := feeds{}

	err := f.readFeedsFile("file_doesnt_exist.json")

	if err == nil {
		t.Fatal("expected error calling readFeedsFile on non-existent file", err)
	}
}
