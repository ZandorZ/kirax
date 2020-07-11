package example

import (
	"testing"
	"time"

	"github.com/ZandorZ/kirax"
	"github.com/r3labs/diff"
	"syreclabs.com/go/faker"

	"github.com/matryer/is"
)

func Test_Dispatch(t *testing.T) {

	is := is.New(t)

	user := CreateNewUser()
	store := kirax.NewStore(user)

	err := store.AddModifier("CHANGE_NAME", func(state *User, name string) error {
		state.Name = name
		return nil
	})
	is.NoErr(err)

	err = store.AddModifier("ADD_AGE", func(state *User, age int) error {
		state.Age += age
		return nil
	})
	is.NoErr(err)

	t.Run("Current state has not changed yet", func(t *testing.T) {
		curState, err := store.GetState()
		is.NoErr(err)
		is.Equal(user.Age, curState.(User).Age)
	})

	t.Run("Add age to user", func(t *testing.T) {
		curState, err := store.GetState()
		is.NoErr(err)

		plusAge := 10
		err = store.Dispatch(kirax.Action{Name: "ADD_AGE", Payload: plusAge})
		is.NoErr(err)

		newState, err := store.GetState()
		is.NoErr(err)

		expected := curState.(User).Age + plusAge
		is.Equal(newState.(User).Age, expected)
	})

	t.Run("Change user name", func(t *testing.T) {

		newName := faker.Name().Name()
		store.Dispatch(kirax.Action{Name: "CHANGE_NAME", Payload: newName})

		newState, err := store.GetState()
		is.NoErr(err)
		is.Equal(newState.(User).Name, newName)

	})

}

func Test_Custom_Store(t *testing.T) {

	is := is.New(t)

	userStore := NewUserStore(make(map[string]User))

	err := userStore.InitStore()
	is.NoErr(err)

	t.Run("New cities", func(t *testing.T) {

		for i := 0; i < 5; i++ {
			err := userStore.AddUser(CreateNewUser())
			is.NoErr(err)
			time.Sleep(300 * time.Millisecond)
		}

		cities, err := userStore.GetStateByPath("Users.Address.City")
		is.NoErr(err)

		diffs, err := diff.Diff(cities, userStore.Cities)
		is.True(len(diffs) == 0)
	})

}
