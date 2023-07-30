package rssole

import (
	"log"
	"os"
	"testing"
	"time"
)

var tempDir string

func feedsSetUpTearDown(_ *testing.T) func(t *testing.T) {
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
		log.Println(err)

		return
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
		log.Println(err)

		return
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

func TestAddFeed(t *testing.T) {
	f := feeds{
		UpdateTime: 1 * time.Second,
	}

	f.addFeed(&feed{})
	f.addFeed(&feed{})

	if len(f.Feeds) != 2 {
		t.Fatal("expected 2 feeds to be added")
	}
}

func TestDelFeed(t *testing.T) {
	f := feeds{
		UpdateTime: 1 * time.Second,
	}

	fd := &feed{URL: "1"}
	f.addFeed(fd)
	f.addFeed(&feed{URL: "2"})
	f.addFeed(&feed{URL: "3"})

	f.delFeed(fd.ID())

	if len(f.Feeds) != 2 {
		t.Fatal("expected 2 feeds to be left")
	}
}

func TestGetFeedByID(t *testing.T) {
	f := feeds{
		UpdateTime: 1 * time.Second,
	}

	f1 := &feed{URL: "1"}
	f2 := &feed{URL: "2"}
	f3 := &feed{URL: "3"}

	f.addFeed(f1)
	f.addFeed(f2)
	f.addFeed(f3)

	found1 := f.getFeedByID(f1.ID())
	found2 := f.getFeedByID(f2.ID())
	found3 := f.getFeedByID(f3.ID())
	found4 := f.getFeedByID("no such id")

	if found1 != f1 {
		t.Fatal("expected to find f1")
	}

	if found2 != f2 {
		t.Fatal("expected to find f2")
	}

	if found3 != f3 {
		t.Fatal("expected to find f3")
	}

	if found4 != nil {
		t.Fatal("expected not to find unadded feed")
	}
}

func TestSaveFeedsFile_Success(t *testing.T) {
	defer feedsSetUpTearDown(t)(t)

	file, err := os.CreateTemp(tempDir, "*")
	if err != nil {
		log.Println(err)

		return
	}

	f := feeds{
		filename: file.Name(),
	}
	expectedFileContents := `{
  "config": {
    "listen": "",
    "update_seconds": 0
  },
  "feeds": null
}
`

	err = f.saveFeedsFile()

	if err != nil {
		t.Fatal("unexpected error calling saveFeedsFile", err)
	}

	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal("unexpected error reading file back in", err)
	}

	if string(data) != expectedFileContents {
		t.Fatal("expected data, got:", string(data))
	}
}
