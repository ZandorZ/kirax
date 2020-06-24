package kirax

import (
	"testing"
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
