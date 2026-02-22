package rssole

import (
	"sync"
	"text/template"
	"time"

	"golang.org/x/exp/slog"
)

// ReadCache provides read/unread tracking for feed items.
type ReadCache interface {
	IsUnread(id string) bool
	MarkRead(id string)
	ExtendLifeIfFound(id string)
	Persist()
}

// ActivityTracker tracks client activity and last-modified state.
type ActivityTracker interface {
	IsIdle() bool
	UpdateLastModified()
}

// Service holds all the state for an rssole instance.
// This allows multiple instances to run in parallel (useful for testing).
type Service struct {
	// Core state
	feeds     *feeds
	readLut   *unreadLut
	templates map[string]*template.Template

	// Activity tracking (for idle detection)
	lastActivity   time.Time
	lastActivityMu sync.Mutex
	startOnce      sync.Once

	// Last modified tracking (for HTTP caching)
	lastmodified   time.Time
	muLastmodified sync.Mutex
}

// NewService creates a new Service instance with initialized state.
func NewService() *Service {
	return &Service{
		feeds:     &feeds{list: newFeedList()},
		readLut:   &unreadLut{},
		templates: nil, // loaded via loadTemplates
	}
}

// Ensure Service implements ActivityTracker.
var _ ActivityTracker = (*Service)(nil)

// UpdateLastModified updates the last modified timestamp.
func (s *Service) UpdateLastModified() {
	s.muLastmodified.Lock()
	s.lastmodified = time.Now()
	s.muLastmodified.Unlock()
}

// getLastmodified returns the last modified timestamp.
func (s *Service) getLastmodified() time.Time {
	s.muLastmodified.Lock()
	defer s.muLastmodified.Unlock()

	return s.lastmodified
}

// recordActivity records client activity and triggers feed updates if needed.
func (s *Service) recordActivity() {
	s.startOnce.Do(func() {
		slog.Info("First client connected, starting feed updates")
		s.feeds.BeginFeedUpdates(s.readLut, s)
	})

	var wasIdle bool

	s.lastActivityMu.Lock()
	wasIdle = !s.lastActivity.IsZero() && time.Since(s.lastActivity) > idleTimeout
	s.lastActivity = time.Now()
	s.lastActivityMu.Unlock()

	if wasIdle {
		slog.Info("Client reconnected after idle, triggering feed updates")
		s.feeds.triggerUpdates()
	}
}

// IsIdle returns true if no client activity has occurred recently.
func (s *Service) IsIdle() bool {
	s.lastActivityMu.Lock()
	defer s.lastActivityMu.Unlock()

	if s.lastActivity.IsZero() {
		return false
	}

	return time.Since(s.lastActivity) > idleTimeout
}
