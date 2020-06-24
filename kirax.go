package kirax

import (
	"log"
	"sync"
	"time"

	"github.com/jinzhu/copier"
	"github.com/mcuadros/go-lookup"
	"github.com/r3labs/diff"
)

// Store ...
type Store struct {
	state interface{}
	sync.RWMutex
	listeners Listener
}

// Listener ...
type Listener map[string]chan SnapShot

// SnapShot ...
type SnapShot struct {
	time.Time
	Data interface{}
}

// NewStore ...
func NewStore(initState interface{}) *Store {
	return &Store{
		state:     initState,
		listeners: make(map[string]chan SnapShot),
	}
}

// PatchState ...
func (s *Store) PatchState(path string, payload interface{}) (err error) {
	// TODO
	return nil
}

// SetState ...
func (s *Store) SetState(payload interface{}) error {
	s.Lock()
	defer s.Unlock()

	var wg sync.WaitGroup

	///////////////////////////////////////////////////
	for path, listener := range s.listeners {
		wg.Add(1)
		go s.worker(path, &wg, listener, payload)
	}

	wg.Wait()

	copier.Copy(&s.state, &payload)
	///////////////////////////////////////////////////

	return nil
}

// AddListener ...
func (s *Store) AddListener(path string) <-chan SnapShot {
	s.Lock()
	defer s.Unlock()
	if s.listeners[path] == nil {
		s.listeners[path] = make(chan SnapShot)
	}
	return s.listeners[path]
}

// GetStateByPath return the state by given path
func (s *Store) GetStateByPath(path string) (interface{}, error) {
	s.RLock()
	defer s.RUnlock()
	value, err := lookup.LookupString(s.state, path)
	return value.Interface(), err
}

// GetState return current state.
func (s *Store) GetState() interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.state
}

// worker internal
func (s *Store) worker(path string, wg *sync.WaitGroup, listener chan<- SnapShot, payload interface{}) {

	defer wg.Done()

	oldV, _ := lookup.LookupString(s.state, path)
	newV, _ := lookup.LookupString(payload, path)

	d, err := diff.NewDiffer(diff.SliceOrdering(true))
	if err != nil {
		log.Fatal(err)
	}
	changes, err := d.Diff(oldV.Interface(), newV.Interface())
	if err != nil {
		log.Fatal(err)
	}

	if len(changes) > 0 {
		listener <- SnapShot{Time: time.Now(), Data: newV.Interface()}
	}

}
