package kirax

import (
	"log"
	"math/rand"
	"testing"
	"time"
)

type Address struct {
	City   string
	Street string
	Number int
}

type Customer struct {
	FirstName, LastName string
	Age                 int
	Address             Address
}

type CustomerCollection []Customer

type CustomerStore struct {
	*Store
}

func (c *CustomerStore) GetState() Customer {
	return c.Store.GetState().(Customer)
}

func NewCustomerStore(c Customer) *CustomerStore {
	return &CustomerStore{NewStore(c)}
}

var c = Customer{
	FirstName: "John",
	LastName:  "Doe",
	Age:       30,
	Address: Address{
		City:   "New York",
		Street: "5th Av",
		Number: 14,
	},
}

func Test_SetState(t *testing.T) {

	store := NewCustomerStore(c)

	newState := store.GetState()
	newState.Age++

	store.SetState(newState)

	curState := store.GetState()
	if curState.Age != 31 {
		t.Errorf("Age wrong. Expected 31 got: %d", curState.Age)
	}

}

func Test_GetStateSlice(t *testing.T) {

	store := NewStore(CustomerCollection{c, c})

	list2 := store.AddListener(".")

	done := make(chan bool)
	ticker1 := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker1.C {
			c.Address.Number += rand.Intn(10)
			store.SetState(CustomerCollection{c})
		}
	}()

	go func() {

		for snap := range list2 {

			if snap.Error != nil {
				t.Error(snap.Error)
			} else {
				log.Println("Changed : ", snap.Data.(CustomerCollection))
				if snap.Data.(CustomerCollection)[0].Address.Number > 40 {
					done <- true
				}
			}
		}

	}()

	<-done

}
