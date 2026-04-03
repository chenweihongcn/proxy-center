package session

import "sync"

type ActiveCount struct {
	Username string `json:"username"`
	Count    int    `json:"count"`
}

type entry struct {
	username string
	closer   func()
}

type Manager struct {
	mu      sync.Mutex
	nextID  uint64
	active  map[string]int
	entries map[uint64]entry
	byUser  map[string]map[uint64]struct{}
}

func NewManager() *Manager {
	return &Manager{
		active:  make(map[string]int),
		entries: make(map[uint64]entry),
		byUser:  make(map[string]map[uint64]struct{}),
	}
}

func (m *Manager) Acquire(username string, maxConns int, closer func()) (uint64, bool, int) {
	if maxConns <= 0 {
		maxConns = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	curr := m.active[username]
	if curr >= maxConns {
		return 0, false, curr
	}
	m.nextID++
	id := m.nextID
	m.active[username] = curr + 1
	m.entries[id] = entry{username: username, closer: closer}
	if _, ok := m.byUser[username]; !ok {
		m.byUser[username] = make(map[uint64]struct{})
	}
	m.byUser[username][id] = struct{}{}
	return id, true, curr + 1
}

func (m *Manager) SetCloser(id uint64, closer func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return
	}
	e.closer = closer
	m.entries[id] = e
}

func (m *Manager) Release(id uint64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return 0
	}
	delete(m.entries, id)
	if userEntries, ok := m.byUser[e.username]; ok {
		delete(userEntries, id)
		if len(userEntries) == 0 {
			delete(m.byUser, e.username)
		}
	}

	curr := m.active[e.username]
	if curr <= 1 {
		delete(m.active, e.username)
		return 0
	}
	m.active[e.username] = curr - 1
	return curr - 1
}

func (m *Manager) Snapshot() []ActiveCount {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ActiveCount, 0, len(m.active))
	for user, cnt := range m.active {
		out = append(out, ActiveCount{Username: user, Count: cnt})
	}
	return out
}

func (m *Manager) ActiveUsernames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	users := make([]string, 0, len(m.active))
	for user := range m.active {
		users = append(users, user)
	}
	return users
}

func (m *Manager) KickUser(username string) int {
	m.mu.Lock()
	ids := m.byUser[username]
	closers := make([]func(), 0, len(ids))
	for id := range ids {
		if e, ok := m.entries[id]; ok && e.closer != nil {
			closers = append(closers, e.closer)
		}
	}
	m.mu.Unlock()

	for _, c := range closers {
		c()
	}
	return len(closers)
}
