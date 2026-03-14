package resume

import (
	"sync"
)

type UserState struct {
	baseResume            *string
	currentOptimized      *string
	currentJobDescription *string
}

type State struct {
	mu    sync.RWMutex
	users map[string]*UserState
}

func NewState() *State {
	return &State{
		users: make(map[string]*UserState),
	}
}

func (s *State) getOrCreateLocked(userID string) *UserState {
	state, ok := s.users[userID]
	if !ok {
		state = &UserState{}
		s.users[userID] = state
	}
	return state
}

func (s *State) SetBaseResume(userID, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.getOrCreateLocked(userID)
	v := value
	state.baseResume = &v
	state.currentOptimized = nil
}

func (s *State) SetOptimizedResume(userID, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.getOrCreateLocked(userID)
	v := value
	state.currentOptimized = &v
}

func (s *State) GetBaseResume(userID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.users[userID]
	if !ok || state.baseResume == nil {
		return "", false
	}
	return *state.baseResume, true
}

func (s *State) GetActiveResume(userID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.users[userID]
	if !ok {
		return "", false
	}
	if state.currentOptimized != nil {
		return *state.currentOptimized, true
	}
	if state.baseResume != nil {
		return *state.baseResume, true
	}
	return "", false
}

func (s *State) SetJobDescription(userID, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.getOrCreateLocked(userID)
	v := value
	state.currentJobDescription = &v
	state.currentOptimized = nil
}

func (s *State) GetJobDescription(userID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.users[userID]
	if !ok || state.currentJobDescription == nil {
		return "", false
	}
	return *state.currentJobDescription, true
}

