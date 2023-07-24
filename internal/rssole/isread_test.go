package rssole

import (
	"os"
	"testing"
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
