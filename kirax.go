package kirax

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/mcuadros/go-lookup"
	"github.com/modern-go/reflect2"
	"github.com/r3labs/diff"
	"github.com/reactivex/rxgo/v2"
)

// Action ...
type Action struct {
	Name    string
	Payload interface{}
}

// SnapShot ...
type SnapShot struct {
	time.Time
	Data  interface{}
	Error error
}

// Listener ...
type Listener chan SnapShot

// Modifier ...
type Modifier func(state interface{}, payload interface{})

// Store ...
type Store struct {
	sync.RWMutex
	state           reflect.Value // reflect.Ptr
	listeners       map[string]Listener
	modifiers       map[string]Modifier
	modifierMethods map[string]reflect.Value
	stateCh         chan rxgo.Item
}

// NewStore ...
func NewStore(initState interface{}) *Store {
	return &Store{
		state:           reflect.ValueOf(initState), // TODO: check if struct or pointer to struct
		stateCh:         make(chan rxgo.Item),
		listeners:       make(map[string]Listener),
		modifiers:       make(map[string]Modifier),
		modifierMethods: make(map[string]reflect.Value),
	}
}

// AddListener ...
func (s *Store) AddListener(path string) <-chan SnapShot {
	// TODO: check if path exist in state struct
	if _, ok := s.listeners[path]; !ok {
		s.listeners[path] = make(chan SnapShot)
	}
	return s.listeners[path]
}

// AddModifier ...
func (s *Store) AddModifier(action string, modifier Modifier) error {
	if _, ok := s.modifiers[action]; !ok {
		s.modifiers[action] = modifier
	}
	return nil
	// TODO modifier already exists
	// TODO add more than 1 modifier per action
}

// AddModifierMethod ...
func (s *Store) AddModifierMethod(action string, modifier interface{}) error {
	mod := reflect.ValueOf(modifier)
	if !mod.IsValid() {
		return fmt.Errorf("Modifier is not valid")
	}
	if mod.Kind().String() != "func" {
		return fmt.Errorf("Modifier is not a func")
	}
	s.Lock()
	defer s.Unlock()
	if _, ok := s.modifiers[action]; !ok {
		s.modifierMethods[action] = mod
	}
	return nil
}

// NewObservable creates new state observable
func (s *Store) NewObservable() rxgo.Observable {
	return rxgo.FromChannel(s.stateCh, rxgo.WithPublishStrategy())
}

//Dispatch ...
func (s *Store) Dispatch(action Action) error {
	s.Lock()
	defer s.Unlock()

	oldV, err := s.getStateV()
	if err != nil {
		return err
	}

	if mod, ok := s.modifiers[action.Name]; ok {
		mod(s.state.Interface(), action.Payload)
		go func() { s.stateCh <- rxgo.Of(s.state.Interface()) }()
		return s.checkState(oldV)
	}

	return fmt.Errorf("Action '%s' not found", action.Name)
}

// Dispatch2 ...
func (s *Store) Dispatch2(action Action) error {

	if mod, ok := s.modifierMethods[action.Name]; ok {

		if action.Payload == nil && mod.Type().NumIn() > 0 {
			return fmt.Errorf("Calling '%s' with payload nil, but asking %d argument(s)", mod.Type(), mod.Type().NumIn())
		}

		if action.Payload != nil && mod.Type().NumIn() == 0 {
			return fmt.Errorf("Calling '%s' with unnecessary payload", mod.Type())
		}

		//TODO: check argument type/kind
		// log.Printf("modifier first argument type: %v", mod.Type().In(0).Kind())
		// log.Printf("payload type: %v", reflect.TypeOf(action.Payload).Kind())

		if action.Payload != nil {
			args := []reflect.Value{s.state, reflect.ValueOf(action.Payload)}
			mod.Call(args)
		} else {
			mod.Call([]reflect.Value{s.state})
		}
		go func() { s.stateCh <- rxgo.Of(s.state.Elem().Interface()) }()
		return nil //TODO checkState()

	}
	return fmt.Errorf("Action '%s' not found", action.Name)
}

// getValeu reflectValue of state copy
func (s *Store) getStateV() (reflect.Value, error) {

	state := s.state.Elem().Interface()

	v := reflect2.TypeOf(state).New()

	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	bytes, err := json.Marshal(state)
	if err != nil {
		return reflect.Value{}, err
	}

	err = json.Unmarshal(bytes, v)
	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(v), nil

}

// GetState copies the current state
func (s *Store) GetState() (interface{}, error) {
	state, err := s.getStateV()
	if err != nil {
		return nil, err
	}
	return state.Elem().Interface(), err
}

// checkState warns listeners for changes
func (s *Store) checkState(oldState reflect.Value) error {

	var wg sync.WaitGroup

	for path, listener := range s.listeners {
		wg.Add(1)
		go s.worker(path, &wg, listener, oldState)
	}

	wg.Wait()

	return nil
}

// worker ...
func (s *Store) worker(path string, wg *sync.WaitGroup, listener chan<- SnapShot, oldState reflect.Value) {

	defer wg.Done()

	oldV := oldState
	newV := s.state

	if path != "/" {

		var err error

		oldV, err = lookup.LookupString(oldState.Interface(), path)
		if err != nil {
			listener <- SnapShot{Error: fmt.Errorf("Error finding path in old state: %v", oldV)}
			return
		}

		newV, err = lookup.LookupString(s.state.Interface(), path)
		if err != nil {
			listener <- SnapShot{Error: fmt.Errorf("Error finding path in new state: %v", newV)}
			return
		}
	}

	d, err := diff.NewDiffer(diff.SliceOrdering(true))
	if err != nil {
		listener <- SnapShot{Error: err}
		return
	}

	changes, err := d.Diff(oldV.Interface(), newV.Interface())
	if err != nil {
		listener <- SnapShot{Error: err}
		return
	}

	if len(changes) > 0 {
		listener <- SnapShot{Time: time.Now(), Data: newV.Interface()}
	}

	return

}
