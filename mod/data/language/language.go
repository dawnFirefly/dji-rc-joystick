package language

import (
	"errors"
	"fmt"
	"sync"

	targetlanguage "github.com/voluminor/dji-rc-joystick/target/language"
)

// // // // // // // // // //

const fallbackKey = "en"

// ErrKeyEmpty reports a missing language key.
var ErrKeyEmpty = errors.New("language key is empty")

// OptionObj describes one selectable language.
type OptionObj struct {
	// Key is the stable locale identifier used by Set and New.
	Key string
	// Name is the language's display name in that language.
	Name string
}

// Obj provides concurrent access to the active translation catalog.
type Obj struct {
	current string
	mx      sync.RWMutex
}

// New selects key as the active language.
func New(key string) (*Obj, error) {
	if key == "" {
		return nil, ErrKeyEmpty
	}

	if !targetlanguage.Supports(key) {
		return nil, fmt.Errorf("unsupported language: %s", key)
	}

	return &Obj{current: key}, nil
}

// List returns a newly allocated list of supported languages.
func (obj *Obj) List() []OptionObj {
	keys := targetlanguage.Keys()
	result := make([]OptionObj, 0, len(keys))
	for _, key := range keys {
		name, ok := targetlanguage.Name(key)
		if !ok {
			continue
		}
		result = append(result, OptionObj{Key: key, Name: name})
	}

	return result
}

// Supports reports whether key identifies an embedded language.
func (obj *Obj) Supports(key string) bool {
	return targetlanguage.Supports(key)
}

// Current returns the active language key.
func (obj *Obj) Current() string {
	obj.mx.RLock()
	defer obj.mx.RUnlock()

	return obj.current
}

// Set changes the active language.
func (obj *Obj) Set(key string) error {
	if key == "" {
		return ErrKeyEmpty
	}
	if !obj.Supports(key) {
		return fmt.Errorf("unsupported language: %s", key)
	}

	obj.mx.Lock()
	obj.current = key
	obj.mx.Unlock()
	return nil
}

// Get resolves id in the active language, then English, then fallback.
func (obj *Obj) Get(id, fallback string) string {
	obj.mx.RLock()
	current := obj.current
	obj.mx.RUnlock()

	if value, ok := targetlanguage.Get(current, id); ok {
		return value
	}
	if value, ok := targetlanguage.Get(fallbackKey, id); ok {
		return value
	}

	return fallback
}
