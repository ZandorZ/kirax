package kirax

import (
	"fmt"
	"log"
	"testing"
	"time"

	"syreclabs.com/go/faker"
)

type Address struct {
	Number int
	Street string
	City   string
}

type User struct {
	Name, ID string
	Age      int
	Address  Address
	Tags     []string
}

func randomUser() User {
	return User{
		ID:   faker.Code().Ean8(),
		Name: faker.Name().Name(),
		Age:  faker.Number().NumberInt(2),
	}
}

var user = &User{
	ID:   "8888888888",
	Name: "Zandor",
	Age:  40,
	Address: Address{
		City:   "New York",
		Street: "5th Avenue",
		Number: 10,
	},
	Tags: []string{"dev", "linux"},
}

func Test_Dispatch(t *testing.T) {

	store := NewStore(user)

	mod1 := func(state interface{}, payload interface{}) {
		state.(*User).Age += payload.(int)
	}
	store.AddModifier("ADD_AGE", mod1)

	mod2 := func(state *User, payload string) {
		state.Name = payload
	}
	store.AddModifierMethod("CHANGE_NAME", mod2)

	curState, err := store.GetState()
	if err != nil {
		t.Fatal(err)
	}

	if curState.(User).Age != 40 {
		t.Errorf("Age state invalid. Expected '%d', got '%d'", 40, curState.(User).Age)
	}

	plusAge := 10
	store.Dispatch(Action{Name: "ADD_AGE", Payload: plusAge})

	newState, err := store.GetState()
	if err != nil {
		t.Fatal(err)
	}

	expected := curState.(User).Age + plusAge
	if newState.(User).Age != expected {
		t.Errorf("Age state invalid after dispatch. Expected '%d', got '%d'", expected, curState.(User).Age)
	}

	newName := "John"
	store.Dispatch2(Action{Name: "CHANGE_NAME", Payload: newName})

	newState, err = store.GetState()
	if err != nil {
		t.Fatal(err)
	}

	if newState.(User).Name != newName {
		t.Errorf("Name state invalid after dispatch. Expected '%s', got '%s'", newName, curState.(User).Name)
	}

}

func Test_Listeners(t *testing.T) {

	store := NewStore(user)

	mod1 := func(state interface{}, payload interface{}) {
		state.(*User).Address.City = payload.(string)
	}
	store.AddModifier("UPDATE_ADDRESS_CITY", mod1)

	list1 := store.AddListener("Address")

	ticker := time.NewTicker(1 * time.Second)

	done := make(chan bool)

	go func() {
		for range ticker.C {
			err := store.Dispatch(Action{Name: "UPDATE_ADDRESS_CITY", Payload: faker.Address().City()})
			if err != nil {
				fmt.Println("erro")
			}
		}
	}()

	go func() {
		count := 0
		for snap := range list1 {
			if snap.Error != nil {
				log.Fatal(snap.Error)
				done <- true
			}
			log.Printf("Adress changed: %v", snap.Data)
			count++
			if count > 5 {
				done <- true
			}
		}
	}()

	<-done

}

func Test_Observable(t *testing.T) {

	store := NewStore(user)

	mod := func(state *User, payload int) {
		state.Age = payload
	}
	store.AddModifierMethod("CHANGE_AGE", mod)

	onlyAdults := store.NewObservable().
		Filter(func(item interface{}) bool {
			return item.(User).Age > 18
		})

	// Create the first Observer
	onlyAdults.DoOnNext(func(data interface{}) {
		fmt.Printf("First observer: Age: %d\n", data.(User).Age)
		if data.(User).Age <= 18 {
			t.Errorf("Only users older than 18 are allowed. Got: %d", data.(User).Age)
		}
	})

	// Create the second Observer
	onlyAdults.DoOnNext(func(data interface{}) {
		fmt.Printf("Second observer: Age: %d\n", data.(User).Age)
	})

	// onComplete
	onlyAdults.DoOnCompleted(func() {
		fmt.Println("Observable is closed:")
	})

	onlyAdults.DoOnError(func(err error) {
		fmt.Printf("Observable error: %v", err)
	})

	context, dispose := onlyAdults.Connect()

	go func() {
		for i := 0; i < 10; i++ {
			err := store.Dispatch2(Action{Name: "CHANGE_AGE", Payload: faker.Number().NumberInt(2)})
			if err != nil {
				fmt.Printf("error dispatching: %v\n", err)
			}
			time.Sleep(250 * time.Millisecond)
		}
		dispose()
	}()

	// Wait for the subscription to be done
	<-context.Done()

	fmt.Println("Done:")

}

func Test_Iterable(t *testing.T) {

}
