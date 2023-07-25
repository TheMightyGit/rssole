package rssole

import (
	"os"
	"testing"
	"time"
)

func TestIsUnread(t *testing.T) {
	dir, err := os.MkdirTemp("", "Test_IsUnread")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	file, err := os.CreateTemp(dir, "*")
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(file.Name(), []byte(`{"persisted_read":"2023-07-21T18:11:29.802432+01:00"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	readLut1 := unreadLut{
		Filename: file.Name(),
	}

	readLut1.loadReadLut()

	readLut1.markRead("this_is_read")

	if readLut1.isUnread("persisted_read") {
		t.Fatal("this_is_read should be read")
	}
	if readLut1.isUnread("this_is_read") {
		t.Fatal("this_is_read should be read")
	}
	if !readLut1.isUnread("this_is_unread") {
		t.Fatal("this_is_unread should be unread")
	}

	readLut1.persistReadLut()

	// unpon reload the same things should still be true...

	readLut2 := unreadLut{
		Filename: file.Name(),
	}

	readLut2.loadReadLut()

	if readLut2.isUnread("persisted_read") {
		t.Fatal("this_is_read should be read after reloading")
	}
	if readLut2.isUnread("this_is_read") {
		t.Fatal("this_is_read should be read after reloading")
	}
	if !readLut2.isUnread("this_is_unread") {
		t.Fatal("this_is_unread should be unread after reloading")
	}
}

func TestRemoveOld(t *testing.T) {
	readLut := unreadLut{
		lut: map[string]time.Time{
			"something_old": time.Now().Add(-61 * time.Hour * 24), // 61 days old
			"something_new": time.Now().Add(-59 * time.Hour * 24), // 59 days old
		},
	}

	if readLut.isUnread("something_old") {
		t.Fatal("something_old should exist before cleanup")
	}
	if readLut.isUnread("something_new") {
		t.Fatal("something_new should exist before cleanup")
	}

	before := time.Now().Add(-60 * time.Hour * 24) // 60 days
	readLut.removeOldEntries(before)

	if !readLut.isUnread("something_old") {
		t.Fatal("something_old should no longer be present after cleanup")
	}
	if readLut.isUnread("something_new") {
		t.Fatal("something_new should exist after cleanup")
	}
}
