package mxj

import (
	"errors"
	"strings"
)

// RenameKey renames a key in a Map.
// It works only for nested maps. 
// It doesn't work for cases when the key is in a list.
func (mv Map) RenameKey(path string, newName string) error {
	var v bool
	var err error
	if v, err = mv.Exists(path); err == nil && !v {
		return errors.New("RenameKey: path not found: " + path)
	} else if err != nil {
		return err
	}
	if v, err = mv.Exists(parentPath(path) + "." + newName); err == nil && v {
		return errors.New("RenameKey: key already exists: " + newName)
	} else if err != nil {
		return err
	}

	m := map[string]interface{}(mv)
	return renameKey(m, path, newName)
}

func renameKey(m interface{}, path string, newName string) error {
	val, err := prevValueByPath(m, path)
	if err != nil {
		return err
	}

	oldName := lastKey(path)
	val[newName] = val[oldName]
	delete(val, oldName)

	return nil
}

// returns a value which contains a last key in the path
// For example: prevValueByPath("a.b.c", {a{b{c: 3}}}) returns {c: 3}
func prevValueByPath(m interface{}, path string) (map[string]interface{}, error) {
	keys := strings.Split(path, ".")

	switch mValue := m.(type) {
	case map[string]interface{}:
		for key, value := range mValue {
			if key == keys[0] {
				if len(keys) == 1 {
					return mValue, nil
				} else {
					// keep looking for the full path to the key
					return prevValueByPath(value, strings.Join(keys[1:], "."))
				}
			}
		}
	}
	return nil, errors.New("prevValueByPath: didn't find path – " + path)
}
