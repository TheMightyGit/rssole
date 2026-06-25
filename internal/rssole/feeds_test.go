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

	return func(_ *testing.T) {
		os.RemoveAll(tempDir)
	}
}

// mockReadCache is a no-op ReadCache for tests that don't need real caching.
type mockReadCache struct{}

func (m *mockReadCache) IsUnread(_ string) bool     { return true }
func (m *mockReadCache) MarkRead(_ string)          {}
func (m *mockReadCache) ExtendLifeIfFound(_ string) {}
func (m *mockReadCache) Persist()                   {}

// mockActivityTracker is a no-op ActivityTracker for tests.
type mockActivityTracker struct{}

func (m *mockActivityTracker) IsIdle() bool        { return false }
func (m *mockActivityTracker) UpdateLastModified() {}

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
		list:       newFeedList(),
	}

	mockRC := &mockReadCache{}
	mockAT := &mockActivityTracker{}

	f1 := &feed{}
	f1.Init()

	f2 := &feed{}
	f2.Init()

	f.addFeed(f1, mockRC, mockAT)
	f.addFeed(f2, mockRC, mockAT)

	// Clean up tickers
	f1.StopTickedUpdate()
	f2.StopTickedUpdate()

	if len(f.All()) != 2 {
		t.Fatal("expected 2 feeds to be added")
	}
}

func TestDelFeed(t *testing.T) {
	f := feeds{
		UpdateTime: 1 * time.Second,
		list:       newFeedList(),
	}

	mockRC := &mockReadCache{}
	mockAT := &mockActivityTracker{}

	fd1 := &feed{URL: "1"}
	fd1.Init()

	fd2 := &feed{URL: "2"}
	fd2.Init()

	fd3 := &feed{URL: "3"}
	fd3.Init()

	f.addFeed(fd1, mockRC, mockAT)
	f.addFeed(fd2, mockRC, mockAT)
	f.addFeed(fd3, mockRC, mockAT)

	f.delFeed(fd1.ID())

	// Clean up remaining tickers
	fd2.StopTickedUpdate()
	fd3.StopTickedUpdate()

	if len(f.All()) != 2 {
		t.Fatal("expected 2 feeds to be left")
	}
}

func TestGetFeedByID(t *testing.T) {
	f := feeds{
		UpdateTime: 1 * time.Second,
		list:       newFeedList(),
	}

	mockRC := &mockReadCache{}
	mockAT := &mockActivityTracker{}

	f1 := &feed{URL: "1"}
	f1.Init()

	f2 := &feed{URL: "2"}
	f2.Init()

	f3 := &feed{URL: "3"}
	f3.Init()

	f.addFeed(f1, mockRC, mockAT)
	f.addFeed(f2, mockRC, mockAT)
	f.addFeed(f3, mockRC, mockAT)

	// Clean up tickers
	defer f1.StopTickedUpdate()
	defer f2.StopTickedUpdate()
	defer f3.StopTickedUpdate()

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
		list:     newFeedList(),
	}
	expectedFileContents := `{
  "config": {
    "listen": "",
    "update_seconds": 0
  },
  "feeds": []
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
