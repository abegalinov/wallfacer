package store

// subscribe registers a channel that receives a signal whenever task state changes.
// The caller must call unsubscribe with the returned ID when done.
func (s *Store) subscribe() (int, <-chan struct{}) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	id := s.nextSubID
	s.nextSubID++
	ch := make(chan struct{}, 1)
	s.subscribers[id] = ch
	return id, ch
}

// Subscribe is the exported variant of subscribe for use outside the package.
func (s *Store) Subscribe() (int, <-chan struct{}) {
	return s.subscribe()
}

func (s *Store) Unsubscribe(id int) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	delete(s.subscribers, id)
}

// notify wakes all SSE subscribers. Non-blocking: if a subscriber's buffer is
// already full it already has a pending signal, so no additional send is needed.
func (s *Store) notify() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
