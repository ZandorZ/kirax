package kirax

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/mcuadros/go-lookup"
	"github.com/r3labs/diff"
)

// Action ...
type Action struct {
	Name    string
	Payload interface{}
}

// SnapShot ...
type SnapShot struct {
	When  time.Time
	Data  interface{}
	Error error
}

// Listener ...
type Listener chan SnapShot

// Store ...
type Store struct {
	sync.RWMutex
	state           reflect.Value // pointer of struct
	cacheState      interface{}
	listeners       map[string]Listener
	modifierMethods map[string]reflect.Value
}

// NewStore creates a new store based on a state
func NewStore(initState interface{}) *Store {

	stateKind := reflect.TypeOf(initState).Kind()

	//only struct
	if stateKind != reflect.Struct {
		panic(fmt.Errorf("Type %s not allowed, state can only be struct", stateKind.String()))
	}

	//clone initial state
	clonedInitial, err := Clone(initState)
	if err != nil {
		panic(err)
	}

	return &Store{
		state:           clonedInitial,
		cacheState:      initState,
		listeners:       make(map[string]Listener),
		modifierMethods: make(map[string]reflect.Value),
	}
}

// AddListener ...
func (s *Store) AddListener(path string) <-chan SnapShot {
	if _, ok := s.listeners[path]; !ok {
		s.listeners[path] = make(chan SnapShot)
	}
	return s.listeners[path]
}

// RemoveListener ...
func (s *Store) RemoveListener(path string) error {
	if _, ok := s.listeners[path]; ok {
		close(s.listeners[path])
		delete(s.listeners, path)
		return nil
	}
	return fmt.Errorf("Listener to '%s' not registered", path)
}

// AddModifier ...
func (s *Store) AddModifier(action string, modifier interface{}) error {

	if _, ok := s.modifierMethods[action]; ok {
		return fmt.Errorf("Modifier '%s' already registered", action)
	}

	modV := reflect.ValueOf(modifier)

	if !modV.IsValid() {
		return fmt.Errorf("Modifier is not valid")
	}

	if reflect.TypeOf(modifier).Kind() != reflect.Func {
		return fmt.Errorf("Modifier is not a function")
	}

	// no arguments or first argument is not same type of state
	if modV.Type().NumIn() == 0 || (modV.Type().NumIn() > 0 && modV.Type().In(0) != s.state.Type()) {
		return fmt.Errorf("Modifier '%s' needs 1st argument to be of type: '%s'", modV.Type().String(), s.state.Type().String())
	}

	if _, ok := s.modifierMethods[action]; !ok {
		s.modifierMethods[action] = modV
	}

	return nil
}

// Dispatch actions
func (s *Store) Dispatch(action Action) error {

	if mod, ok := s.modifierMethods[action.Name]; ok {

		if action.Payload == nil && mod.Type().NumIn() > 1 {
			return fmt.Errorf("Calling modifier '%s' with payload nil, but asking %d argument(s)", mod.Type(), mod.Type().NumIn())
		}

		if action.Payload != nil && mod.Type().NumIn() == 1 {
			return fmt.Errorf("Calling modifier '%s' with unnecessary payload: %v", mod.Type(), action.Payload)
		}

		oldV, err := s.getStateV()
		if err != nil {
			return err
		}

		s.Lock()
		defer s.Unlock()

		if action.Payload != nil {
			args := []reflect.Value{s.state, reflect.ValueOf(action.Payload)}
			mod.Call(args)
		} else {
			mod.Call([]reflect.Value{s.state})
		}

		//clear cache
		s.cacheState = nil

		//check changes
		s.checkState(oldV)

		return nil
	}

	return fmt.Errorf("Action '%s' not registered", action.Name)
}

// getValue reflectValue of state copy
func (s *Store) getStateV() (reflect.Value, error) {

	//do I need this?
	s.RLock()
	defer s.RUnlock()

	var err error
	var stateV reflect.Value
	if s.cacheState == nil {
		stateV, err = Clone(s.state.Elem().Interface())
		s.cacheState = stateV.Elem().Interface()
	}

	return reflect.ValueOf(s.cacheState), err
}

// GetState copies the current state
func (s *Store) GetState() (interface{}, error) {
	state, err := s.getStateV()
	if err != nil {
		return nil, err
	}
	return state.Interface(), err
}

// GetStateByPath copies the current state path
func (s *Store) GetStateByPath(path string) (interface{}, error) {
	state, err := s.getStateV()
	if err != nil {
		return nil, err
	}
	statePath, err := lookup.LookupString(state.Interface(), path)
	if err != nil {
		return nil, err
	}
	return statePath.Interface(), nil
}

// checkState warns listeners for changes
func (s *Store) checkState(oldState reflect.Value) {

	var wg sync.WaitGroup
	for path, listener := range s.listeners {
		wg.Add(1)
		go s.worker(path, &wg, listener, oldState)
	}

	wg.Wait()
}

// worker ...
func (s *Store) worker(path string, wg *sync.WaitGroup, listener chan<- SnapShot, oldState reflect.Value) {

	defer wg.Done()

	oldV := oldState
	newV := s.state

	// not root
	if path != "/" {
		var err error
		oldV, err = lookup.LookupString(oldState.Interface(), path)
		if err != nil {
			s.noBlocking(listener, SnapShot{Error: fmt.Errorf("Error finding path in old state: %v", oldV)})
			return
		}

		newV, err = lookup.LookupString(s.state.Interface(), path)
		if err != nil {
			s.noBlocking(listener, SnapShot{Error: fmt.Errorf("Error finding path in new state: %v", newV)})
			return
		}
	}

	d, err := diff.NewDiffer(diff.SliceOrdering(true))
	if err != nil {
		s.noBlocking(listener, SnapShot{Error: err})
		return
	}

	changes, err := d.Diff(oldV.Interface(), newV.Interface())
	if err != nil {
		s.noBlocking(listener, SnapShot{Error: err})
		return
	}

	if len(changes) > 0 {
		s.noBlocking(listener, SnapShot{When: time.Now(), Data: newV.Interface()})
	}

	return
}

// noBlocking sending to channel listener
func (s *Store) noBlocking(listener chan<- SnapShot, snap SnapShot) {
	select {
	case listener <- snap:
	default:
	}
}
