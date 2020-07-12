package example

import (
	"fmt"

	"github.com/ZandorZ/kirax"
	"github.com/google/uuid"
	"syreclabs.com/go/faker"
)

// ADD_USER Action name
const ADD_USER = "AddUser"

// Address ...
type Address struct {
	Street string
	City   string
}

// User ..
type User struct {
	Name, ID string
	Age      int
	Address  Address
	Tags     []string
}

// CreateNewUser creates a random user
func CreateNewUser() User {
	return User{
		ID:   uuid.New().String(),
		Name: faker.Name().Name(),
		Age:  faker.RandomInt(18, 100),
		Address: Address{
			City:   faker.Address().City(),
			Street: faker.Address().StreetAddress(),
		},
		Tags: faker.Lorem().Words(faker.RandomInt(1, 4)),
	}
}

// UserStore custom store
type UserStore struct {
	*kirax.Store
}

// UserState ...
type UserState struct {
	Users      map[string]User
	SelectedID string
	Cities []string
}

// NewUserStore creates new store
func NewUserStore(users map[string]User) *UserStore {
	return &UserStore{Store: kirax.NewStore(UserState{Users: users})}
}

// InitStore initializes the store with listeners and modifiers
func (u *UserStore) InitStore() error {
	if err := u.AddModifier(ADD_USER, u.onAddUser); err != nil {
		return err
	}
	go u.listenNewCities(u.AddListener("Users.Address.City"))
	return nil
}

// listenNewCities channel listener to new cities snapshots
func (u *UserStore) listenNewCities(ch <-chan kirax.SnapShot) {
	for snap := range ch {		
		u.Patch("Cities", snap.Data.([]string)) 
	}
}

// onAddUser is a modifier to add user to state
func (u *UserStore) onAddUser(state *UserState, newUser User) error {
	if _, ok := state.Users[newUser.ID]; !ok {
		state.Users[newUser.ID] = newUser
		return nil
	}
	return fmt.Errorf("User ID %s already exists", newUser.ID)
}

// GetState casts UserState type
func (u *UserStore) GetState() (UserState, error) {
	state, err := u.Store.GetState()
	return state.(UserState), err
}

// AddUser invokes dipatch action ADD_USER
func (u *UserStore) AddUser(user User) error {
	return u.Dispatch(kirax.Action{Name: ADD_USER, Payload: user})
}
