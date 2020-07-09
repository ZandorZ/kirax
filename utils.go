package kirax

import (
	"bytes"
	"encoding/gob"
	"reflect"

	"github.com/modern-go/reflect2"
)

// Clone a value
func Clone(target interface{}) (reflect.Value, error) {

	v := reflect2.TypeOf(target).New()

	b := new(bytes.Buffer)
	err := gob.NewEncoder(b).Encode(target)
	if err != nil {
		return reflect.Value{}, err
	}

	data := b.Bytes()
	b = bytes.NewBuffer(data)
	err = gob.NewDecoder(b).Decode(v)
	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(v), nil
}
