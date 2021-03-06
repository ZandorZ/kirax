package kirax

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/mcuadros/go-lookup"
	"github.com/r3labs/diff"
)

// Action to be dispatched
type Action struct {
	Name    string
	Payload interface{}
}

// SnapShot snapshots of values
type SnapShot struct {
	When  time.Time
	Data  interface{}
	Error error
}

// Listener channel to SnapShot changes
type Listener chan SnapShot

// Store main struct
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

// AddListener returns a channel to changes according to a path
func (s *Store) AddListener(path string) <-chan SnapShot {
	if _, ok := s.listeners[path]; !ok {
		s.listeners[path] = make(chan SnapShot)
	}
	return s.listeners[path]
}

// RemoveListener removes and close the listener channel
func (s *Store) RemoveListener(path string) error {
	if _, ok := s.listeners[path]; ok {
		close(s.listeners[path])
		delete(s.listeners, path)
		return nil
	}
	return fmt.Errorf("Listener to '%s' not registered", path)
}

// AddModifier adds a func that modifies the state, according to a Action name
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

	// no return error
	errorInterface := reflect.TypeOf((*error)(nil)).Elem()
	if modV.Type().NumOut() == 0 || !modV.Type().Out(0).Implements(errorInterface) {
		return fmt.Errorf("Modifier '%s' must return type: error", modV.Type().String())
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

// Patch state according to a path
func (s *Store) Patch(path string, payload interface{}) error {
	s.Lock()
	defer s.Unlock()

	oldV, err := s.getStateV()
	if err != nil {
		return err
	}

	targetV, err := lookup.LookupString(s.state.Interface(), path)
	if err != nil {
		return fmt.Errorf("Invalid path '%s'", path)
	}

	//types are different
	if reflect.TypeOf(payload) != targetV.Type() {
		return fmt.Errorf("Target path and payload have different types")
	}

	if targetV.CanSet() {

		targetV.Set(reflect.ValueOf(payload))

		//clear cache
		s.cacheState = nil

		//check changes
		s.checkState(oldV)
	}

	return err
}

// Dispatch actions to modify the state
func (s *Store) Dispatch(action Action) error {

	s.Lock()
	defer s.Unlock()

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

		var returnedError interface{}
		if action.Payload != nil {
			args := []reflect.Value{s.state, reflect.ValueOf(action.Payload)}
			returnedError = mod.Call(args)[0].Interface()
		} else {
			returnedError = mod.Call([]reflect.Value{s.state})[0].Interface()
		}

		//clear cache
		s.cacheState = nil
		//check changes
		s.checkState(oldV)

		if returnedError != nil {
			return returnedError.(error)
		}

		return nil
	}

	return fmt.Errorf("Action '%s' not registered", action.Name)
}

// getValue reflectValue of state copy
func (s *Store) getStateV() (reflect.Value, error) {

	var err error
	var stateV reflect.Value
	if s.cacheState == nil {
		stateV, err = Clone(s.state.Elem().Interface())
		s.cacheState = stateV.Elem().Interface()
	}
	return reflect.ValueOf(s.cacheState), err
}

// GetState get copy of the current state
func (s *Store) GetState() (interface{}, error) {
	s.Lock()
	defer s.Unlock()
	state, err := s.getStateV()
	if err != nil {
		return nil, err
	}
	return state.Interface(), err
}

// GetStateByPath get copies of the current state according to a path
func (s *Store) GetStateByPath(path string) (interface{}, error) {
	s.Lock()
	defer s.Unlock()
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

// worker to check changes and warn listeners by their path
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
